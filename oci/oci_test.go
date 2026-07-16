package oci

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/registry"

	"github.com/cocoonstack/cocoon-common/manifest"
	"github.com/cocoonstack/cocoon-common/snapshot"
)

// TestOCIRegistryRoundTrip exercises the full Registry surface against an
// in-memory OCI registry: a blob and a custom-artifactType manifest survive a
// put -> exists -> get -> delete round trip.
func TestOCIRegistryRoundTrip(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	r := NewOCIRegistry(strings.TrimPrefix(srv.URL, "http://")+"/cocoon", authn.DefaultKeychain)
	ctx := t.Context()

	blob := []byte("hello cocoon blob")
	sum := sha256.Sum256(blob)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	if ok, err := r.HasBlob(ctx, "myvm", digest); err != nil || ok {
		t.Fatalf("HasBlob before put = (%v, %v), want (false, nil)", ok, err)
	}
	if err := r.PutBlob(ctx, "myvm", digest, bytes.NewReader(blob), int64(len(blob))); err != nil {
		t.Fatalf("PutBlob: %v", err)
	}
	if ok, err := r.HasBlob(ctx, "myvm", digest); err != nil || !ok {
		t.Fatalf("HasBlob after put = (%v, %v), want (true, nil)", ok, err)
	}

	rc, err := r.GetBlob(ctx, "myvm", digest)
	if err != nil {
		t.Fatalf("GetBlob: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()
	if !bytes.Equal(got, blob) {
		t.Fatalf("GetBlob = %q, want %q", got, blob)
	}

	const mt = "application/vnd.oci.image.manifest.v1+json"
	mf := []byte(`{"schemaVersion":2,"mediaType":"` + mt +
		`","artifactType":"application/vnd.cocoonstack.snapshot.v1+json","config":{"mediaType":` +
		`"application/vnd.cocoonstack.snapshot.config.v1+json","digest":"` + digest +
		`","size":` + strconv.Itoa(len(blob)) + `},"layers":[]}`)

	if ok, err := r.HasManifest(ctx, "myvm", "hibernate"); err != nil || ok {
		t.Fatalf("HasManifest before put = (%v, %v), want (false, nil)", ok, err)
	}
	if err := r.PutManifest(ctx, "myvm", "hibernate", mf, mt); err != nil {
		t.Fatalf("PutManifest: %v", err)
	}
	if ok, err := r.HasManifest(ctx, "myvm", "hibernate"); err != nil || !ok {
		t.Fatalf("HasManifest after put = (%v, %v), want (true, nil)", ok, err)
	}
	raw, gotMT, err := r.GetManifest(ctx, "myvm", "hibernate")
	if err != nil {
		t.Fatalf("GetManifest: %v", err)
	}
	if !bytes.Equal(raw, mf) {
		t.Fatalf("GetManifest bytes mismatch")
	}
	if gotMT != mt {
		t.Fatalf("GetManifest mediaType = %q, want %q", gotMT, mt)
	}

	if err := r.DeleteManifest(ctx, "myvm", "hibernate"); err != nil {
		t.Fatalf("DeleteManifest: %v", err)
	}
	if _, _, err := r.GetManifest(ctx, "myvm", "hibernate"); err == nil {
		t.Fatal("GetManifest after delete: want error, got nil")
	}
	if err := r.DeleteManifest(ctx, "myvm", "hibernate"); err != nil {
		t.Fatalf("DeleteManifest of absent tag must be nil (ensure-absent), got %v", err)
	}
}

// TestGetManifestByDigest confirms GetManifest addresses a sha256:... reference
// via '@' (repo@digest), the path snapshot pull takes to fetch a multi-arch
// image-index child — a ':' join here would be rejected by name.ParseReference.
func TestGetManifestByDigest(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	r := NewOCIRegistry(strings.TrimPrefix(srv.URL, "http://")+"/cocoon", authn.DefaultKeychain)
	ctx := t.Context()

	const mt = "application/vnd.oci.image.manifest.v1+json"
	mf := []byte(`{"schemaVersion":2,"mediaType":"` + mt +
		`","artifactType":"application/vnd.cocoonstack.snapshot.v1+json","config":{"mediaType":` +
		`"application/vnd.cocoonstack.snapshot.config.v1+json","digest":"sha256:` +
		strings.Repeat("0", 64) + `","size":0},"layers":[]}`)
	if err := r.PutManifest(ctx, "myvm", "latest", mf, mt); err != nil {
		t.Fatalf("PutManifest: %v", err)
	}

	raw, gotMT, err := r.GetManifest(ctx, "myvm", digestOf(mf))
	if err != nil {
		t.Fatalf("GetManifest by digest: %v", err)
	}
	if !bytes.Equal(raw, mf) {
		t.Fatalf("GetManifest by digest bytes mismatch")
	}
	if gotMT != mt {
		t.Fatalf("GetManifest by digest mediaType = %q, want %q", gotMT, mt)
	}
}

// TestStreamResolvesIndexChildByDigest drives snapshot.Stream through the
// image-index path against a live registry. The child snapshot manifest is
// referenced only by digest, so the stream fails unless the OCIRegistry fetches
// it via repo@digest — the regression this fix targets.
func TestStreamResolvesIndexChildByDigest(t *testing.T) {
	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	r := NewOCIRegistry(strings.TrimPrefix(srv.URL, "http://")+"/cocoon", authn.DefaultKeychain)
	ctx := t.Context()

	cfgBlob := []byte(`{"schemaVersion":"v1","snapshotId":"snap-1","hypervisor":"cloud-hypervisor"}`)
	if err := r.PutBlob(ctx, "myvm", digestOf(cfgBlob), bytes.NewReader(cfgBlob), int64(len(cfgBlob))); err != nil {
		t.Fatalf("PutBlob config: %v", err)
	}
	layerBlob := []byte(`{"cpu":2}`)
	if err := r.PutBlob(ctx, "myvm", digestOf(layerBlob), bytes.NewReader(layerBlob), int64(len(layerBlob))); err != nil {
		t.Fatalf("PutBlob layer: %v", err)
	}

	child := []byte(`{"schemaVersion":2,"mediaType":"` + manifest.MediaTypeOCIManifest +
		`","artifactType":"` + manifest.ArtifactTypeSnapshot +
		`","config":{"mediaType":"` + manifest.MediaTypeSnapshotConfig + `","digest":"` + digestOf(cfgBlob) +
		`","size":` + strconv.Itoa(len(cfgBlob)) + `},"layers":[{"mediaType":"` + manifest.MediaTypeVMConfig +
		`","digest":"` + digestOf(layerBlob) + `","size":` + strconv.Itoa(len(layerBlob)) +
		`,"annotations":{"` + manifest.AnnotationTitle + `":"config.json"}}]}`)
	// Store the child by tag; the registry then serves it back by its digest.
	if err := r.PutManifest(ctx, "myvm", "child", child, manifest.MediaTypeOCIManifest); err != nil {
		t.Fatalf("PutManifest child: %v", err)
	}

	index := []byte(`{"schemaVersion":2,"mediaType":"` + manifest.MediaTypeOCIIndex +
		`","manifests":[{"mediaType":"` + manifest.MediaTypeOCIManifest + `","digest":"` + digestOf(child) +
		`","size":` + strconv.Itoa(len(child)) + `,"platform":{"os":"linux","architecture":"amd64"}}]}`)

	var buf bytes.Buffer
	if err := snapshot.Stream(ctx, index, r, snapshot.StreamOptions{Name: "myvm", Writer: &buf}); err != nil {
		t.Fatalf("Stream index->child: %v", err)
	}

	tr := tar.NewReader(&buf)
	found := false
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read import tar: %v", err)
		}
		if hdr.Name != "config.json" {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read config.json: %v", err)
		}
		if !bytes.Equal(body, layerBlob) {
			t.Fatalf("config.json = %q, want %q", body, layerBlob)
		}
		found = true
	}
	if !found {
		t.Fatal("import tar missing config.json layer from resolved child manifest")
	}
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
