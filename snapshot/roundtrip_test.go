package snapshot

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/cocoonstack/cocoon-common/manifest"
	"github.com/cocoonstack/cocoon-common/ociutil"
)

var v2OptionMatrix = []struct {
	name string
	opts PushOptions
}{
	{"compressed", PushOptions{ZstdLevel: 3}},
	{"chunked", PushOptions{ChunkSizeMiB: 1}},
	{"both", PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1}},
	{"both-k2", PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1, Concurrency: 2}},
}

// Invariant 2: the v2 pipeline must be invisible at the tar layer — its output
// is byte-identical to the v1 pipeline's for the same input.
func TestV2RoundTripMatchesV1ByteForByte(t *testing.T) {
	pinClock(t)
	corpus := v2Corpus(t)
	want := roundTrip(t, corpus, PushOptions{})
	for _, tc := range v2OptionMatrix {
		t.Run(tc.name, func(t *testing.T) {
			got := roundTrip(t, corpus, tc.opts)
			if !bytes.Equal(want, got) {
				t.Fatalf("v2 (%s) import tar differs from v1 import tar: %d vs %d bytes", tc.name, len(got), len(want))
			}
		})
	}
}

// Invariant 1: per-entry equivalence with the source export tar — name, order,
// size, body, mode, and COCOON.sparse.* PAX records. snapshot.json is compared
// semantically since the reader re-marshals the envelope by design.
func TestV2RoundTripPreservesEntries(t *testing.T) {
	pinClock(t)
	corpus := v2Corpus(t)
	source := readTarEntries(t, bytes.NewReader(corpus))
	for _, tc := range v2OptionMatrix {
		t.Run(tc.name, func(t *testing.T) {
			got := readTarEntries(t, bytes.NewReader(roundTrip(t, corpus, tc.opts)))
			if len(got) != len(source) {
				t.Fatalf("entry count = %d, want %d", len(got), len(source))
			}
			for i, want := range source {
				g := got[i]
				if g.name != want.name {
					t.Fatalf("entry %d name = %q, want %q", i, g.name, want.name)
				}
				if want.name == snapshotJSONName {
					var wantEnv, gotEnv snapshotExportEnvelope
					if err := json.Unmarshal(want.body, &wantEnv); err != nil {
						t.Fatalf("decode source envelope: %v", err)
					}
					if err := json.Unmarshal(g.body, &gotEnv); err != nil {
						t.Fatalf("decode round-trip envelope: %v", err)
					}
					if fmt.Sprintf("%+v", gotEnv.Config) != fmt.Sprintf("%+v", wantEnv.Config) {
						t.Errorf("envelope config = %+v, want %+v", gotEnv.Config, wantEnv.Config)
					}
					continue
				}
				if g.size != want.size || !bytes.Equal(g.body, want.body) {
					t.Errorf("%s: size/body mismatch (size %d vs %d)", g.name, g.size, want.size)
				}
				if g.mode != want.mode {
					t.Errorf("%s: mode = %o, want %o", g.name, g.mode, want.mode)
				}
				for k, v := range want.pax {
					if g.pax[k] != v {
						t.Errorf("%s: PAX %s = %q, want %q", g.name, k, g.pax[k], v)
					}
				}
			}
		})
	}
}

func TestV2ManifestShape(t *testing.T) {
	pinClock(t)
	uploader := newFakeUploader()
	pushCorpus(t, uploader, v2Corpus(t), PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1})

	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	parsed, err := manifest.Parse(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if parsed.ArtifactType != manifest.ArtifactTypeSnapshotV2 {
		t.Errorf("artifactType = %q, want %q", parsed.ArtifactType, manifest.ArtifactTypeSnapshotV2)
	}
	if got := manifest.ClassifyParsed(parsed); got != manifest.KindSnapshot {
		t.Errorf("ClassifyParsed = %v, want KindSnapshot", got)
	}

	var cfg manifest.SnapshotConfig
	if err := json.Unmarshal(uploader.blobs[parsed.Config.Digest], &cfg); err != nil {
		t.Fatalf("decode config blob: %v", err)
	}
	if cfg.SchemaVersion != "v2" {
		t.Errorf("schemaVersion = %q, want v2", cfg.SchemaVersion)
	}

	var memChunks []manifest.Descriptor
	for _, l := range parsed.Layers {
		if l.Title() == "memory-ranges" {
			memChunks = append(memChunks, l)
		}
	}
	if len(memChunks) != 3 {
		t.Fatalf("memory-ranges chunk layers = %d, want 3", len(memChunks))
	}
	memMeta := cfg.Files["memory-ranges"]
	if memMeta.Size != 2<<20+512<<10 {
		t.Errorf("memory-ranges files[].size = %d, want %d", memMeta.Size, 2<<20+512<<10)
	}
	if len(memMeta.Chunks) != 3 {
		t.Fatalf("memory-ranges files[].chunks = %d, want 3", len(memMeta.Chunks))
	}
	for i, l := range memChunks {
		if !manifest.IsZstdMediaType(l.MediaType) || manifest.StripZstd(l.MediaType) != manifest.MediaTypeVMMemory {
			t.Errorf("chunk %d mediaType = %q", i, l.MediaType)
		}
		if l.Annotations[manifest.AnnotationChunkIndex] != strconv.Itoa(i) {
			t.Errorf("chunk %d index annotation = %q", i, l.Annotations[manifest.AnnotationChunkIndex])
		}
		if l.Annotations[manifest.AnnotationChunkCount] != "3" {
			t.Errorf("chunk %d count annotation = %q", i, l.Annotations[manifest.AnnotationChunkCount])
		}
		if memMeta.Chunks[i] != l.Digest {
			t.Errorf("files[].chunks[%d] = %q, want %q", i, memMeta.Chunks[i], l.Digest)
		}
	}

	for _, l := range parsed.Layers {
		if l.Title() == "config.json" && l.MediaType != manifest.MediaTypeVMConfig {
			t.Errorf("small file config.json mediaType = %q, want raw %q", l.MediaType, manifest.MediaTypeVMConfig)
		}
	}
}

// Knobs on but nothing compressed or chunked (all files tiny) must still
// produce a v1-classified manifest so phase-0 readers stay compatible.
func TestV2KnobsWithOnlySmallFilesStaysV1(t *testing.T) {
	pinClock(t)
	corpus := buildOrderedExportTar(t, snapshotExportConfig{Name: "myvm"}, []namedTarEntry{
		{"config.json", exportTarEntry{data: []byte(`{}`), mode: 0o640}},
		{"state.json", exportTarEntry{data: []byte(`{"s":1}`), mode: 0o640}},
	})
	uploader := newFakeUploader()
	pushCorpus(t, uploader, corpus, PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1})

	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	parsed, err := manifest.Parse(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if parsed.ArtifactType != manifest.ArtifactTypeSnapshot {
		t.Errorf("artifactType = %q, want v1 %q", parsed.ArtifactType, manifest.ArtifactTypeSnapshot)
	}
}

func TestPullFailsClosedOnUnknownMediaType(t *testing.T) {
	pinClock(t)
	uploader := newFakeUploader()
	pushCorpus(t, uploader, v2Corpus(t), PushOptions{})

	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	mutated := bytes.Replace(raw, []byte(manifest.MediaTypeVMMemory), []byte("application/vnd.cocoonstack.vm.memory+future"), 1)
	err = Stream(t.Context(), mutated, uploader, StreamOptions{Name: "myvm", Writer: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "unsupported mediaType") {
		t.Fatalf("err = %v, want unsupported mediaType", err)
	}
}

func TestPullFailsClosedOnEncodedLayerInV1Manifest(t *testing.T) {
	pinClock(t)
	uploader := newFakeUploader()
	pushCorpus(t, uploader, v2Corpus(t), PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1})

	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	mutated := bytes.ReplaceAll(raw, []byte(manifest.ArtifactTypeSnapshotV2), []byte(manifest.ArtifactTypeSnapshot))
	err = Stream(t.Context(), mutated, uploader, StreamOptions{Name: "myvm", Writer: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "but manifest is not") {
		t.Fatalf("err = %v, want encoded-layer-in-v1 rejection", err)
	}
}

func TestPullFailsClosedOnMissingFileSize(t *testing.T) {
	pinClock(t)
	uploader := newFakeUploader()
	pushCorpus(t, uploader, v2Corpus(t), PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1})

	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	parsed, err := manifest.Parse(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	var cfg manifest.SnapshotConfig
	if err := json.Unmarshal(uploader.blobs[parsed.Config.Digest], &cfg); err != nil {
		t.Fatalf("decode config blob: %v", err)
	}
	meta := cfg.Files["memory-ranges"]
	meta.Size = 0
	cfg.Files["memory-ranges"] = meta
	mutatedCfg, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	uploader.blobs[parsed.Config.Digest] = mutatedCfg

	err = Stream(t.Context(), raw, uploader, StreamOptions{Name: "myvm", Writer: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "missing files[].size") {
		t.Fatalf("err = %v, want missing files[].size", err)
	}
}

func TestPullFailsClosedOnMissingChunkLayer(t *testing.T) {
	pinClock(t)
	uploader := newFakeUploader()
	pushCorpus(t, uploader, v2Corpus(t), PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1})

	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	parsed, err := manifest.Parse(raw)
	if err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	var cfg manifest.SnapshotConfig
	if err := json.Unmarshal(uploader.blobs[parsed.Config.Digest], &cfg); err != nil {
		t.Fatalf("decode config blob: %v", err)
	}
	meta := cfg.Files["memory-ranges"]
	meta.Chunks[1] = "sha256:deadbeef"
	cfg.Files["memory-ranges"] = meta
	mutatedCfg, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	uploader.blobs[parsed.Config.Digest] = mutatedCfg

	err = Stream(t.Context(), raw, uploader, StreamOptions{Name: "myvm", Writer: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "missing from manifest layers") {
		t.Fatalf("err = %v, want missing from manifest layers", err)
	}
}

// Identical chunks dedup across files into one blob carrying one file's
// annotations; reconstruction must not reject the other file's reference.
func TestRoundTripSharedChunkAcrossFiles(t *testing.T) {
	pinClock(t)
	const chunk = 1 << 20
	shared := fillBytes(chunk, 7)
	corpus := buildOrderedExportTar(t, snapshotExportConfig{Name: "myvm"}, []namedTarEntry{
		{"memory-ranges", exportTarEntry{data: append(fillBytes(chunk, 1), shared...), mode: 0o600}},
		{"overlay.qcow2", exportTarEntry{data: append(fillBytes(chunk, 2), shared...), mode: 0o640}},
	})
	for _, tc := range v2OptionMatrix {
		t.Run(tc.name, func(t *testing.T) {
			want := roundTrip(t, corpus, PushOptions{})
			got := roundTrip(t, corpus, tc.opts)
			if !bytes.Equal(want, got) {
				t.Fatalf("shared-chunk corpus: v2 (%s) differs from v1 output", tc.name)
			}
		})
	}
}

// The v2 reader must keep consuming the literal v1 shape — spelled as strings,
// not constants, so constant drift cannot silently rewrite what "v1" means.
func TestReaderConsumesFrozenV1Manifest(t *testing.T) {
	pinClock(t)
	uploader := newFakeUploader()

	memBody := []byte("MEMBYTES")
	cfgBlob := []byte(`{"schemaVersion":"v1","snapshotId":"snap-frozen","hypervisor":"cloud-hypervisor",` +
		`"files":{"memory-ranges":{"mode":384,"sparseMap":"[{\"o\":0,\"l\":4}]","sparseSize":16}}}`)
	memDigest := "sha256:" + ociutil.SHA256Hex(memBody)
	cfgDigest := "sha256:" + ociutil.SHA256Hex(cfgBlob)
	uploader.blobs[memDigest] = memBody
	uploader.blobs[cfgDigest] = cfgBlob
	raw := fmt.Appendf(nil, `{
		"schemaVersion": 2,
		"mediaType": "application/vnd.oci.image.manifest.v1+json",
		"artifactType": "application/vnd.cocoonstack.snapshot.v1+json",
		"config": {"mediaType": "application/vnd.cocoonstack.snapshot.config.v1+json", "digest": %q, "size": %d},
		"layers": [{
			"mediaType": "application/vnd.cocoonstack.vm.memory",
			"digest": %q, "size": %d,
			"annotations": {"org.opencontainers.image.title": "memory-ranges"}
		}]
	}`, cfgDigest, len(cfgBlob), memDigest, len(memBody))

	var buf bytes.Buffer
	if err := Stream(t.Context(), raw, uploader, StreamOptions{Name: "myvm", Writer: &buf}); err != nil {
		t.Fatalf("Stream frozen v1 manifest: %v", err)
	}
	for _, e := range readTarEntries(t, &buf) {
		if e.name != "memory-ranges" {
			continue
		}
		if !bytes.Equal(e.body, memBody) || e.mode != 0o600 || e.pax[sparsePAXMap] != `[{"o":0,"l":4}]` || e.pax[sparsePAXSize] != "16" {
			t.Fatalf("frozen v1 reconstruction mismatch: %+v", e)
		}
		return
	}
	t.Fatal("memory-ranges entry not reconstructed")
}

// Negative knobs must sanitize to defaults, not zero out the buffer pools.
func TestPushSanitizesNegativeConcurrency(t *testing.T) {
	pinClock(t)
	corpus := v2Corpus(t)
	want := roundTrip(t, corpus, PushOptions{})
	for _, opts := range []PushOptions{
		{ChunkSizeMiB: 1, Concurrency: -1},
		{ZstdLevel: 3, ChunkSizeMiB: 1, Concurrency: -5},
		{ZstdLevel: 3, ChunkSizeMiB: 1, MemoryBudgetMiB: -100},
	} {
		if got := roundTrip(t, corpus, opts); !bytes.Equal(want, got) {
			t.Fatalf("opts %+v: output differs from v1", opts)
		}
	}
}

func TestPushRejectsChunkLargerThanBudget(t *testing.T) {
	pinClock(t)
	// One worker needs 2 pools × 2 buffers = 4×chunk; below that, reject.
	// Above maxChunkSizeMiB, reject before any budget math.
	for _, tc := range []struct {
		chunkMiB, budgetMiB int
		wantErr             string
	}{
		{8192, 1024, "maximum"},
		{1 << 50, 0, "maximum"},      // would overflow the byte shift
		{512, 1024, "memory budget"}, // 2×chunk fits but 4×chunk does not
		{512, 2048, ""},              // exactly one worker's floor
	} {
		uploader := newFakeUploader()
		pusher := &Pusher{Uploader: uploader, Cocoon: &fakeCocoon{exportTar: v2Corpus(t)}}
		_, err := pusher.Push(t.Context(), PushOptions{Name: "myvm", ChunkSizeMiB: tc.chunkMiB, MemoryBudgetMiB: tc.budgetMiB})
		if tc.wantErr == "" && err != nil {
			t.Errorf("chunk %d budget %d: unexpected error %v", tc.chunkMiB, tc.budgetMiB, err)
		}
		if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("chunk %d budget %d: err = %v, want %q", tc.chunkMiB, tc.budgetMiB, err, tc.wantErr)
		}
	}
}

// A budget below two chunks routes the pull to the O(1) streaming path; the
// output must stay byte-identical.
func TestPullTinyBudgetStreamsSequentially(t *testing.T) {
	pinClock(t)
	corpus := v2Corpus(t)
	want := roundTrip(t, corpus, PushOptions{})
	uploader := newFakeUploader()
	pushCorpus(t, uploader, corpus, PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1})
	got := pullTar(t, uploader, StreamOptions{MemoryBudgetMiB: 1})
	if !bytes.Equal(want, got) {
		t.Fatal("tiny-budget pull differs from v1 output")
	}
}

// The window must be sized by the file's largest chunk: a compressible prefix
// must never authorize a window the dense middle can blow past the budget with.
func TestPrefetchWindowBoundedByLargestChunk(t *testing.T) {
	sized := func(sizes ...int64) []manifest.Descriptor {
		descs := make([]manifest.Descriptor, len(sizes))
		for i, s := range sizes {
			descs[i] = manifest.Descriptor{Digest: fmt.Sprintf("sha256:%064d", i), Size: s}
		}
		return descs
	}
	for _, tc := range []struct {
		name     string
		descs    []manifest.Descriptor
		prefetch int
		budget   int64
		want     int
	}{
		{"tiny prefix, dense tail, budget holds none", sized(2, 2, 100, 100, 100, 100), 8, 10, 0},
		{"prefix sum fits but window x max would not", sized(2, 100, 100), 8, 202, 2},
		{"budget holds two largest", sized(2, 2, 100, 100, 100, 100), 8, 200, 2},
		{"homogeneous small capped by chunk count", sized(2, 2, 2, 2), 8, 1 << 20, 4},
		{"homogeneous small capped by prefetch", sized(2, 2, 2, 2, 2, 2, 2, 2, 2, 2), 8, 1 << 20, 8},
		{"single chunk streams", sized(100), 8, 1 << 20, 0},
		{"oversized chunk streams", sized(2, maxBufferedChunkBytes+1), 8, 1 << 40, 0},
		{"all zero-size streams", sized(0, 0, 0), 8, 1 << 20, 0},
	} {
		if got := prefetchWindow(tc.descs, tc.prefetch, tc.budget); got != tc.want {
			t.Errorf("%s: window = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// Futures must enter the queue before their fetch spawns: with window 2 and no
// consumer yet, exactly one fetch may start (queue cap 1, second send blocks).
func TestChunkSourceSpawnsWithinWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := &gatedDownloader{blobs: map[string][]byte{}, release: make(chan struct{})}
		var descs []manifest.Descriptor
		var want []byte
		for i := range 4 {
			b := []byte{byte(i), byte(i + 1)}
			d := "sha256:" + ociutil.SHA256Hex(b)
			g.blobs[d] = b
			descs = append(descs, manifest.Descriptor{Digest: d, Size: 2})
			want = append(want, b...)
		}
		src := newChunkSource(t.Context(), g, "myvm", descs, 2)

		// After Wait every goroutine is durably blocked, so started is final.
		synctest.Wait()
		// Errorf: Fatalf would leak the gated fetches and re-panic synctest.
		if n := g.started.Load(); n != 1 {
			t.Errorf("fetches started before any consumption = %d, want 1", n)
		}
		close(g.release)
		var out bytes.Buffer
		if _, err := io.Copy(&out, src); err != nil {
			t.Fatalf("drain: %v", err)
		}
		if !bytes.Equal(out.Bytes(), want) {
			t.Fatalf("out = %v, want %v", out.Bytes(), want)
		}
	})
}

func TestPullRejectsNegativeLayerSize(t *testing.T) {
	pinClock(t)
	uploader := newFakeUploader()
	pushCorpus(t, uploader, v2Corpus(t), PushOptions{})
	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := manifest.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Layers[2].Size = -1
	mutated, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	err = Stream(t.Context(), mutated, uploader, StreamOptions{Name: "myvm", Writer: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "negative size") {
		t.Fatalf("err = %v, want negative-size rejection (not a panic)", err)
	}
}

// chunkStream must enforce the blob contract itself: digest match, exact size,
// no trailing bytes — driven directly so the zstd decoder can't mask the checks.
func TestChunkStreamVerifiesBlobs(t *testing.T) {
	body := []byte("chunk stream body bytes")
	digest := "sha256:" + ociutil.SHA256Hex(body)
	uploader := newFakeUploader()
	uploader.blobs[digest] = body

	read := func(descs []manifest.Descriptor) error {
		s := &chunkStream{ctx: t.Context(), dl: uploader, name: "myvm", descs: descs}
		_, err := io.Copy(io.Discard, s)
		return err
	}

	if err := read([]manifest.Descriptor{{Digest: digest, Size: int64(len(body))}}); err != nil {
		t.Fatalf("clean blob: %v", err)
	}
	wrongDigest := "sha256:" + ociutil.SHA256Hex([]byte("other"))
	uploader.blobs[wrongDigest] = body
	if err := read([]manifest.Descriptor{{Digest: wrongDigest, Size: int64(len(body))}}); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("err = %v, want digest mismatch", err)
	}
	if err := read([]manifest.Descriptor{{Digest: digest, Size: int64(len(body)) + 5}}); err == nil || !strings.Contains(err.Error(), "shorter than") {
		t.Fatalf("err = %v, want shorter-than-size", err)
	}
	if err := read([]manifest.Descriptor{{Digest: digest, Size: int64(len(body)) - 5}}); err == nil || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("err = %v, want longer-than-size", err)
	}
}

// Re-pushing identical content must be a pure HasBlob no-op (chunk-level dedup).
func TestV2SecondPushSkipsAllBlobs(t *testing.T) {
	pinClock(t)
	corpus := v2Corpus(t)
	uploader := &countingUploader{fakeUploader: newFakeUploader()}
	pushCorpus(t, uploader, corpus, PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1})
	first := uploader.putBlobCalls.Load()
	pushCorpus(t, uploader, corpus, PushOptions{ZstdLevel: 3, ChunkSizeMiB: 1})
	if delta := uploader.putBlobCalls.Load() - first; delta != 0 {
		t.Errorf("second push uploaded %d blobs, want 0", delta)
	}
}

// v2Corpus exercises every codec branch: empty, small-raw, exactly-at-chunk-size,
// one-byte-over, multi-chunk sparse, and an unknown-name generic layer.
func v2Corpus(t *testing.T) []byte {
	t.Helper()
	const chunk = 1 << 20
	entries := []namedTarEntry{
		{"config.json", exportTarEntry{data: []byte(`{"cpu":4}`), mode: 0o640}},
		{"state.json", exportTarEntry{data: nil, mode: 0o640}},
		{"memory-ranges", exportTarEntry{
			data: fillBytes(2*chunk+chunk/2, 1),
			mode: 0o600,
			pax: map[string]string{
				sparsePAXMap:  `[{"o":0,"l":1048576},{"o":4194304,"l":1572864}]`,
				sparsePAXSize: strconv.Itoa(8 * chunk),
			},
		}},
		{"overlay.qcow2", exportTarEntry{data: fillBytes(chunk, 2), mode: 0o640}},
		{"cidata.img", exportTarEntry{data: fillBytes(chunk+1, 3), mode: 0o755}},
		{"extra.bin", exportTarEntry{data: fillBytes(300<<10, 4), mode: 0o640}},
	}
	cfg := snapshotExportConfig{
		ID:         "snap-v2",
		Name:       "myvm",
		Hypervisor: "cloud-hypervisor",
		CPU:        4,
		Memory:     8 << 30,
	}
	return buildOrderedExportTar(t, cfg, entries)
}

type namedTarEntry struct {
	name  string
	entry exportTarEntry
}

func buildOrderedExportTar(t *testing.T, cfg snapshotExportConfig, entries []namedTarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	envelope := snapshotExportEnvelope{Version: 1, Config: cfg}
	envBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if err := tw.WriteHeader(&tar.Header{Name: snapshotJSONName, Size: int64(len(envBytes)), Mode: 0o644}); err != nil {
		t.Fatalf("write envelope header: %v", err)
	}
	if _, err := tw.Write(envBytes); err != nil {
		t.Fatalf("write envelope: %v", err)
	}
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Size: int64(len(e.entry.data)), Mode: e.entry.mode, PAXRecords: e.entry.pax}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write %s header: %v", e.name, err)
		}
		if _, err := tw.Write(e.entry.data); err != nil {
			t.Fatalf("write %s: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return buf.Bytes()
}

// fillBytes is deterministic and half-compressible: zero runs mixed with an
// xorshift stream, so zstd neither trivially collapses nor stores it raw.
func fillBytes(n int, seed uint64) []byte {
	out := make([]byte, n)
	state := seed*2685821657736338717 + 1
	for i := range out {
		if i%512 < 256 {
			continue // zero run
		}
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		out[i] = byte(state)
	}
	return out
}

func pinClock(t *testing.T) {
	t.Helper()
	restore := nowFunc
	nowFunc = func() time.Time { return time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { nowFunc = restore })
}

func pushCorpus(t *testing.T, uploader Uploader, corpus []byte, opts PushOptions) *PushResult {
	t.Helper()
	pusher := &Pusher{Uploader: uploader, Cocoon: &fakeCocoon{exportTar: corpus}}
	opts.Name, opts.Tag = "myvm", "v2test"
	result, err := pusher.Push(t.Context(), opts)
	if err != nil {
		t.Fatalf("Push(%+v): %v", opts, err)
	}
	return result
}

func pullTar(t *testing.T, uploader *fakeUploader, streamOpts StreamOptions) []byte {
	t.Helper()
	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	var buf bytes.Buffer
	streamOpts.Name = "myvm"
	streamOpts.Writer = &buf
	if err := Stream(t.Context(), raw, uploader, streamOpts); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	return buf.Bytes()
}

func roundTrip(t *testing.T, corpus []byte, opts PushOptions) []byte {
	t.Helper()
	uploader := newFakeUploader()
	pushCorpus(t, uploader, corpus, opts)
	return pullTar(t, uploader, StreamOptions{})
}

type tarEntry struct {
	name string
	size int64
	mode int64
	pax  map[string]string
	body []byte
}

func readTarEntries(t *testing.T, r io.Reader) []tarEntry {
	t.Helper()
	var out []tarEntry
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read tar body: %v", err)
		}
		sparse := map[string]string{}
		for k, v := range hdr.PAXRecords {
			if strings.HasPrefix(k, "COCOON.sparse.") {
				sparse[k] = v
			}
		}
		out = append(out, tarEntry{name: hdr.Name, size: hdr.Size, mode: hdr.Mode, pax: sparse, body: body})
	}
}

// gatedDownloader blocks every GetBlob until released and counts starts, so
// tests can observe how many fetches the prefetcher launches.
type gatedDownloader struct {
	blobs   map[string][]byte
	release chan struct{}
	started atomic.Int64
}

func (g *gatedDownloader) GetManifest(context.Context, string, string) ([]byte, string, error) {
	return nil, "", errors.New("unused")
}

func (g *gatedDownloader) GetBlob(_ context.Context, _, digest string) (io.ReadCloser, error) {
	g.started.Add(1)
	<-g.release
	return io.NopCloser(bytes.NewReader(g.blobs[digest])), nil
}

type countingUploader struct {
	*fakeUploader
	putBlobCalls atomic.Int64
}

func (c *countingUploader) PutBlob(ctx context.Context, name, digest string, body io.Reader, size int64) error {
	c.putBlobCalls.Add(1)
	return c.fakeUploader.PutBlob(ctx, name, digest, body, size)
}
