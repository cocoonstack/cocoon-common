package snapshot

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
)

const abortWindow = 4

func TestChunkSourceJoinsInFlightFetchesOnError(t *testing.T) {
	plan := planOf("memory-ranges", false, 8<<10, 1<<10, 1<<10, 1<<10, 1<<10, 1<<10, 1<<10, 1<<10, 1<<10)
	dl := &stallingDownloader{failing: plan.chunks[0].Digest, gate: make(chan struct{}), want: abortWindow - 1}
	pipe := &chunkPipeline{dl: dl, name: "abort", window: abortWindow, outputCap: 1 << 10, out: newBufPool(abortWindow + 1)}

	if _, err := newChunkSource(t.Context(), pipe, plan, abortWindow).WriteTo(io.Discard); err == nil {
		t.Fatal("the chunk-0 failure must surface")
	}
	if held := cap(pipe.out.ch) - len(pipe.out.ch); held != 0 {
		t.Fatalf("%d fetch(es) still hold a buffer after WriteTo returned", held)
	}
}

type stallingDownloader struct {
	failing string
	started atomic.Int64
	gate    chan struct{}
	want    int64
}

func (d *stallingDownloader) GetManifest(context.Context, string, string) ([]byte, string, error) {
	return nil, "", errors.New("unused")
}

func (d *stallingDownloader) GetBlob(ctx context.Context, _, digest string) (io.ReadCloser, error) {
	if digest == d.failing {
		return io.NopCloser(failAfter{gate: d.gate}), nil
	}
	if d.started.Add(1) == d.want {
		close(d.gate)
	}
	return io.NopCloser(stalledBody{ctx: ctx}), nil
}

type failAfter struct{ gate chan struct{} }

func (f failAfter) Read([]byte) (int, error) {
	<-f.gate
	return 0, errors.New("registry 500")
}

type stalledBody struct{ ctx context.Context }

func (s stalledBody) Read([]byte) (int, error) {
	<-s.ctx.Done()
	return 0, s.ctx.Err()
}
