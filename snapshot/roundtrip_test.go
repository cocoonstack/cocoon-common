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
	"testing/iotest"
	"testing/synctest"
	"time"

	"github.com/klauspost/compress/zstd"

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
	for _, tc := range []struct {
		chunkMiB, budgetMiB int
		wantErr             string
	}{
		{8192, 1024, "maximum"},
		{1 << 50, 0, "maximum"},
		{512, 1024, "memory budget"},
		{512, 2048, ""},
	} {
		uploader := newFakeUploader()
		pusher := &Pusher{Uploader: uploader, Cocoon: &fakeCocoon{exportTar: v2Corpus(t)}}
		err := pusher.Push(t.Context(), PushOptions{Name: "myvm", ChunkSizeMiB: tc.chunkMiB, MemoryBudgetMiB: tc.budgetMiB})
		if tc.wantErr == "" && err != nil {
			t.Errorf("chunk %d budget %d: unexpected error %v", tc.chunkMiB, tc.budgetMiB, err)
		}
		if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
			t.Errorf("chunk %d budget %d: err = %v, want %q", tc.chunkMiB, tc.budgetMiB, err, tc.wantErr)
		}
	}
}

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

func TestChunkPipelineWindowBoundedByWidestFile(t *testing.T) {
	for _, tc := range []struct {
		name     string
		plans    []layerPlan
		prefetch int
		budget   int64
		want     int
	}{
		{"tiny prefix, dense tail, budget holds none", []layerPlan{planOf("a", false, 404, 2, 2, 100, 100, 100, 100)}, 8, 10, 0},
		{"current plus two outputs fit exactly", []layerPlan{planOf("a", false, 202, 2, 100, 100)}, 8, 300, 2},
		{"current output does not fit", []layerPlan{planOf("a", false, 202, 2, 100, 100)}, 8, 299, 0},
		{"narrow file must not widen the window", []layerPlan{
			planOf("small", false, 4, 2, 2),
			planOf("dense", false, 400, 100, 100, 100, 100),
		}, 8, 300, 2},
		{"widest file decides even when it comes first", []layerPlan{
			planOf("dense", false, 400, 100, 100, 100, 100),
			planOf("small", false, 4, 2, 2),
		}, 8, 300, 2},
		{"compressed pools fit exactly", []layerPlan{planOf("a", true, 300, 50, 50, 50)}, 8, 550, 2},
		{"compressed pools one short stream", []layerPlan{planOf("a", true, 300, 50, 50, 50)}, 8, 549, 0},
		{"cross-file input and output maxima fit exactly", []layerPlan{
			planOf("wide-output", true, 1800, 100, 100),
			planOf("wide-input", true, 100, 900, 900),
		}, 8, 7200, 2},
		{"cross-file input and output maxima one short stream", []layerPlan{
			planOf("wide-output", true, 1800, 100, 100),
			planOf("wide-input", true, 100, 900, 900),
		}, 8, 7199, 0},
		{"prefetch caps the window", []layerPlan{planOf("a", false, 20, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2)}, 8, 1 << 20, 8},
		{"oversized chunk streams", []layerPlan{planOf("a", false, 1, 2, maxBufferedChunkBytes+1)}, 8, 1 << 40, 0},
		{"all zero-size streams", []layerPlan{planOf("a", false, 0, 0, 0, 0)}, 8, 1 << 20, 0},
		{"whole-layer entries need no buffers", []layerPlan{{title: "a", layer: manifest.Descriptor{Size: 100}}}, 8, 1 << 20, 0},
	} {
		p, err := newChunkPipeline(nil, "myvm", tc.plans, tc.prefetch, tc.budget)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		p.Close()
		if p.window != tc.want {
			t.Errorf("%s: window = %d, want %d", tc.name, p.window, tc.want)
		}
	}
}

func TestChunkPipelineFileWindowNarrowsPerFile(t *testing.T) {
	plans := []layerPlan{planOf("dense", false, 8, 2, 2, 2, 2)}
	p, err := newChunkPipeline(nil, "myvm", plans, 8, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.window != 8 {
		t.Fatalf("shared window = %d, want 8 (prefetch, budget is ample)", p.window)
	}
	for _, tc := range []struct {
		name string
		plan layerPlan
		want int
	}{
		{"fewer chunks than the window", planOf("a", false, 6, 2, 2, 2), 3},
		{"more chunks than the window", planOf("a", false, 20, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2), 8},
		{"single chunk streams", planOf("a", false, 2, 2), 0},
	} {
		if got := p.fileWindow(tc.plan); got != tc.want {
			t.Errorf("%s: fileWindow = %d, want %d", tc.name, got, tc.want)
		}
	}
	var off chunkPipeline
	if got := off.fileWindow(plans[0]); got != 0 {
		t.Errorf("window-less pipeline: fileWindow = %d, want 0", got)
	}
}

func TestRawChunkStrideNeverUnderstatesTheCut(t *testing.T) {
	for _, tc := range []struct {
		stride int64
		n      int
	}{{256 << 20, 32}, {256 << 20, 2}, {1 << 20, 7}, {3, 5}} {
		size := tc.stride*int64(tc.n-1) + 1
		for _, last := range []int64{1, tc.stride / 2, tc.stride} {
			if last == 0 {
				continue
			}
			total := tc.stride*int64(tc.n-1) + last
			if got := rawChunkStride(total, tc.n); got < tc.stride {
				t.Errorf("stride %d n %d last %d: bound %d understates the cut", tc.stride, tc.n, last, got)
			}
		}
		if got := rawChunkStride(size, tc.n); got < tc.stride {
			t.Errorf("stride %d n %d: bound %d understates the cut", tc.stride, tc.n, got)
		}
	}
	if got := rawChunkStride(100, 1); got != 100 {
		t.Errorf("single chunk: got %d, want the whole file", got)
	}
}

func TestChunkPipelineRejectsDecodedChunkOverCap(t *testing.T) {
	raw := bytes.Repeat([]byte("x"), 32)
	enc, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = enc.Close() }()
	stored := enc.EncodeAll(raw, nil)
	digest := "sha256:" + ociutil.SHA256Hex(stored)
	uploader := newFakeUploader()
	uploader.blobs[digest] = stored
	desc := manifest.Descriptor{Digest: digest, Size: int64(len(stored))}
	entry := layerPlan{
		title:  "memory",
		meta:   manifest.SnapshotFile{Size: 8},
		layer:  manifest.Descriptor{MediaType: manifest.ZstdMediaType(manifest.MediaTypeVMMemory)},
		chunks: []manifest.Descriptor{desc, desc},
	}
	budget := int64(8 + 2*(len(stored)+8))
	p, err := newChunkPipeline(uploader, "myvm", []layerPlan{entry}, 2, budget)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	res := p.fetch(t.Context(), desc, true)
	if res.err == nil {
		p.out.put(res.buf)
		t.Fatalf("decoded %d bytes into an 8-byte slot", len(res.data))
	}
	if !errors.Is(res.err, zstd.ErrDecoderSizeExceeded) && !errors.Is(res.err, zstd.ErrWindowSizeExceeded) {
		t.Fatalf("decode error = %v, want a zstd size limit", res.err)
	}
}

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
		pipe := &chunkPipeline{dl: g, name: "myvm", window: 2, outputCap: 2, out: newBufPool(3)}
		src := newChunkSource(t.Context(), pipe, layerPlan{title: "myvm", chunks: descs}, 2)

		synctest.Wait()
		if n := g.started.Load(); n != 1 {
			t.Errorf("fetches started before any consumption = %d, want 1", n)
		}
		close(g.release)
		var out bytes.Buffer
		if _, err := src.WriteTo(&out); err != nil {
			t.Fatalf("drain: %v", err)
		}
		if !bytes.Equal(out.Bytes(), want) {
			t.Fatalf("out = %v, want %v", out.Bytes(), want)
		}
	})
}

func TestChunkSourceRejectsShortWrite(t *testing.T) {
	pool := newBufPool(1)
	buf := pool.take(4)
	copy(buf, "data")
	fut := make(chan chunkFetch, 1)
	fut <- chunkFetch{data: buf[:4], buf: buf}
	futures := make(chan chan chunkFetch, 1)
	futures <- fut
	close(futures)

	src := &chunkSource{futures: futures, pipe: &chunkPipeline{out: pool}}
	written, err := src.WriteTo(shortWriter{})
	if written != 3 || !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("WriteTo = (%d, %v), want (3, %v)", written, err, io.ErrShortWrite)
	}
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

func TestChunkStreamEnforcesBlobLength(t *testing.T) {
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
	if err := read([]manifest.Descriptor{{Digest: digest, Size: int64(len(body)) + 5}}); err == nil || !strings.Contains(err.Error(), "shorter than") {
		t.Fatalf("err = %v, want shorter-than-size", err)
	}
	if err := read([]manifest.Descriptor{{Digest: digest, Size: int64(len(body)) - 5}}); err == nil || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("err = %v, want longer-than-size", err)
	}
}

func TestChunkPipelineEnforcesBlobLength(t *testing.T) {
	body := []byte("buffered chunk body")
	digest := "sha256:" + ociutil.SHA256Hex(body)
	uploader := newFakeUploader()
	uploader.blobs[digest] = body

	p := &chunkPipeline{dl: uploader, name: "myvm", outputCap: int64(len(body) + 5), out: newBufPool(1)}
	fetch := func(size int64) error {
		res := p.fetch(t.Context(), manifest.Descriptor{Digest: digest, Size: size}, false)
		if res.err == nil {
			p.out.put(res.buf)
		}
		return res.err
	}

	if err := fetch(int64(len(body))); err != nil {
		t.Fatalf("clean blob: %v", err)
	}
	if err := fetch(int64(len(body)) + 5); err == nil {
		t.Fatal("short blob accepted")
	}
	if err := fetch(int64(len(body)) - 5); err == nil || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("err = %v, want longer-than-size", err)
	}
	if err := fetch(int64(len(body))); err != nil {
		t.Fatalf("pool starved after the error paths: %v", err)
	}
}

func TestChunkPipelinePropagatesTransportError(t *testing.T) {
	body := []byte("verified body")
	verifyErr := errors.New("transport digest mismatch")
	p := &chunkPipeline{
		dl: terminalErrorDownloader{
			body: body,
			err:  verifyErr,
		},
		name: "myvm",
	}

	_, err := p.read(t.Context(), manifest.Descriptor{
		Digest: "sha256:ignored",
		Size:   int64(len(body)),
	}, make([]byte, len(body)))
	if !errors.Is(err, verifyErr) {
		t.Fatalf("read error = %v, want %v", err, verifyErr)
	}
}

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

func planOf(title string, zstd bool, fileSize int64, sizes ...int64) layerPlan {
	descs := make([]manifest.Descriptor, len(sizes))
	for i, s := range sizes {
		descs[i] = manifest.Descriptor{Digest: fmt.Sprintf("sha256:%s%063d", title, i), Size: s}
	}
	mt := manifest.MediaTypeGeneric
	if zstd {
		mt = manifest.ZstdMediaType(mt)
	}
	return layerPlan{title: title, meta: manifest.SnapshotFile{Size: fileSize}, layer: manifest.Descriptor{MediaType: mt}, chunks: descs}
}

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

func fillBytes(n int, seed uint64) []byte {
	out := make([]byte, n)
	state := seed*2685821657736338717 + 1
	for i := range out {
		if i%512 < 256 {
			continue
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

func pushCorpus(t *testing.T, uploader Uploader, corpus []byte, opts PushOptions) {
	t.Helper()
	pusher := &Pusher{Uploader: uploader, Cocoon: &fakeCocoon{exportTar: corpus}}
	opts.Name, opts.Tag = "myvm", "v2test"
	if err := pusher.Push(t.Context(), opts); err != nil {
		t.Fatalf("Push(%+v): %v", opts, err)
	}
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

type terminalErrorDownloader struct {
	body []byte
	err  error
}

func (d terminalErrorDownloader) GetManifest(context.Context, string, string) ([]byte, string, error) {
	return nil, "", errors.ErrUnsupported
}

func (d terminalErrorDownloader) GetBlob(context.Context, string, string) (io.ReadCloser, error) {
	return io.NopCloser(io.MultiReader(bytes.NewReader(d.body), iotest.ErrReader(d.err))), nil
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	return max(len(p)-1, 0), nil
}

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
