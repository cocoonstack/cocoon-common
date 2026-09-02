package oci

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/cocoonstack/cocoon-common/snapshot"
)

const maxRegistryConnsPerHost = 32

// errBlobUncompressed guards DiffID/Uncompressed: cocoon blobs are opaque bytes and WriteLayer reads only Compressed().
var errBlobUncompressed = errors.New("cocoon blob layers expose only compressed bytes")

var _ Registry = (*OCIRegistry)(nil)

// OCIRegistry is a Registry backed by a standard OCI Distribution registry, using upload sessions and keychain auth.
type OCIRegistry struct {
	base string // registry host + repo prefix, e.g. "asia-docker.pkg.dev/proj/repo"
	opts []remote.Option
}

// NewOCIRegistry roots a client at base, authenticating through keychain.
func NewOCIRegistry(base string, keychain authn.Keychain) *OCIRegistry {
	opts := []remote.Option{
		remote.WithAuthFromKeychain(keychain),
		remote.WithTransport(bulkTransport()),
	}
	// Reuse pins one puller and pusher so the /v2/ ping and token exchange happen once per repo.
	if puller, err := remote.NewPuller(opts...); err == nil {
		opts = append(opts, remote.Reuse(puller))
	}
	if pusher, err := remote.NewPusher(opts...); err == nil {
		opts = append(opts, remote.Reuse(pusher))
	}
	return &OCIRegistry{base: base, opts: opts}
}

// GetManifest fetches raw manifest bytes at repo:tag, or repo@digest when tag is a sha256 digest.
func (r *OCIRegistry) GetManifest(ctx context.Context, repo, tag string) ([]byte, string, error) {
	ref, err := r.parseRef(repo, tag)
	if err != nil {
		return nil, "", err
	}
	desc, err := remote.Get(ref, r.callOpts(ctx)...)
	if err != nil {
		if isNotFound(err) {
			return nil, "", fmt.Errorf("get manifest %s:%s: %w", repo, tag, snapshot.ErrManifestNotFound)
		}
		return nil, "", fmt.Errorf("get manifest %s:%s: %w", repo, tag, err)
	}
	return desc.Manifest, string(desc.MediaType), nil
}

func (r *OCIRegistry) GetBlob(ctx context.Context, repo, digest string) (io.ReadCloser, error) {
	ref, err := name.NewDigest(r.base + "/" + repo + "@" + digest)
	if err != nil {
		return nil, fmt.Errorf("parse digest %s@%s: %w", repo, digest, err)
	}
	layer, err := remote.Layer(ref, r.callOpts(ctx)...)
	if err != nil {
		return nil, fmt.Errorf("get blob %s@%s: %w", repo, digest, err)
	}
	return layer.Compressed()
}

func (r *OCIRegistry) HasBlob(ctx context.Context, repo, digest string) (bool, error) {
	ref, err := name.NewDigest(r.base + "/" + repo + "@" + digest)
	if err != nil {
		return false, fmt.Errorf("parse digest %s@%s: %w", repo, digest, err)
	}
	// remote.Layer is lazy; Size() issues the HEAD that reveals whether it exists.
	layer, err := remote.Layer(ref, r.callOpts(ctx)...)
	if err == nil {
		_, err = layer.Size()
	}
	if err == nil {
		return true, nil
	}
	return false, ignoreNotFound(err, "head blob "+repo+"@"+digest)
}

func (r *OCIRegistry) HasManifest(ctx context.Context, repo, tag string) (bool, error) {
	ref, err := r.parseRef(repo, tag)
	if err != nil {
		return false, err
	}
	if _, err := remote.Head(ref, r.callOpts(ctx)...); err != nil {
		return false, ignoreNotFound(err, "head manifest "+repo+":"+tag)
	}
	return true, nil
}

func (r *OCIRegistry) PutBlob(ctx context.Context, repo, digest string, body io.Reader, size int64) error {
	repoRef, err := name.NewRepository(r.base + "/" + repo)
	if err != nil {
		return fmt.Errorf("parse repo %s: %w", repo, err)
	}
	hash, err := v1.NewHash(digest)
	if err != nil {
		return fmt.Errorf("parse digest %s: %w", digest, err)
	}
	if err := remote.WriteLayer(repoRef, &streamLayer{hash: hash, size: size, body: body}, r.callOpts(ctx)...); err != nil {
		return fmt.Errorf("put blob %s@%s: %w", repo, digest, err)
	}
	return nil
}

func (r *OCIRegistry) PutManifest(ctx context.Context, repo, tag string, data []byte, contentType string) error {
	ref, err := r.parseRef(repo, tag)
	if err != nil {
		return err
	}
	if err := remote.Put(ref, rawManifest{data: data, mediaType: types.MediaType(contentType)}, r.callOpts(ctx)...); err != nil {
		return fmt.Errorf("put manifest %s:%s: %w", repo, tag, err)
	}
	return nil
}

// DeleteManifest removes the manifest at repo:reference (tag or digest); a 404 counts as removed.
func (r *OCIRegistry) DeleteManifest(ctx context.Context, repo, reference string) error {
	ref, err := r.parseRef(repo, reference)
	if err != nil {
		return err
	}
	if err := remote.Delete(ref, r.callOpts(ctx)...); err != nil {
		return ignoreNotFound(err, "delete manifest "+repo+":"+reference)
	}
	return nil
}

// parseRef joins repo and reference with '@' for a digest and ':' for a tag.
func (r *OCIRegistry) parseRef(repo, reference string) (name.Reference, error) {
	sep := ":"
	if strings.ContainsRune(reference, ':') {
		sep = "@"
	}
	ref, err := name.ParseReference(r.base + "/" + repo + sep + reference)
	if err != nil {
		return nil, fmt.Errorf("parse ref %s%s%s: %w", repo, sep, reference, err)
	}
	return ref, nil
}

func (r *OCIRegistry) callOpts(ctx context.Context) []remote.Option {
	return append(r.opts, remote.WithContext(ctx))
}

func bulkTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{Proxy: http.ProxyFromEnvironment}
	}
	t := base.Clone()
	t.ForceAttemptHTTP2 = false
	t.TLSClientConfig = &tls.Config{NextProtos: []string{"http/1.1"}, MinVersion: tls.VersionTLS12}
	t.MaxIdleConnsPerHost = maxRegistryConnsPerHost
	return t
}

// ignoreNotFound maps a registry 404 to nil and wraps anything else.
func ignoreNotFound(err error, action string) error {
	if isNotFound(err) {
		return nil
	}
	return fmt.Errorf("%s: %w", action, err)
}

func isNotFound(err error) bool {
	var terr *transport.Error
	return errors.As(err, &terr) && terr.StatusCode == http.StatusNotFound
}

// streamLayer is a single-use v1.Layer over a body of known digest and size, so PutBlob streams without buffering.
type streamLayer struct {
	hash v1.Hash
	size int64
	body io.Reader
}

func (l *streamLayer) Digest() (v1.Hash, error)             { return l.hash, nil }
func (l *streamLayer) Size() (int64, error)                 { return l.size, nil }
func (l *streamLayer) Compressed() (io.ReadCloser, error)   { return io.NopCloser(l.body), nil }
func (l *streamLayer) MediaType() (types.MediaType, error)  { return types.OCILayer, nil }
func (l *streamLayer) DiffID() (v1.Hash, error)             { return v1.Hash{}, errBlobUncompressed }
func (l *streamLayer) Uncompressed() (io.ReadCloser, error) { return nil, errBlobUncompressed }

// rawManifest is a remote.Taggable over pre-serialized manifest bytes.
type rawManifest struct {
	data      []byte
	mediaType types.MediaType
}

func (m rawManifest) RawManifest() ([]byte, error)        { return m.data, nil }
func (m rawManifest) MediaType() (types.MediaType, error) { return m.mediaType, nil }
