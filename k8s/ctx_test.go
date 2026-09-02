package k8s

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestRunTickerRunsUntilCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		calls := 0
		done := make(chan struct{})
		go func() {
			defer close(done)
			RunTicker(ctx, time.Second, func(context.Context) { calls++ })
		}()

		time.Sleep(3*time.Second + time.Millisecond)
		cancel()
		<-done
		if calls != 3 {
			t.Errorf("fn calls = %d, want 3", calls)
		}
	})
}

func TestRunTickerStopsBeforeFirstTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		called := false
		RunTicker(ctx, time.Second, func(context.Context) { called = true })
		if called {
			t.Errorf("fn ran on an already-canceled context")
		}
	})
}

func TestSleepCtxReturnsTrueOnTimer(t *testing.T) {
	if !SleepCtx(t.Context(), time.Millisecond) {
		t.Errorf("timer path should return true")
	}
}

func TestSleepCtxReturnsFalseOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if SleepCtx(ctx, time.Hour) {
		t.Errorf("cancel path should return false")
	}
}

func TestSleepCtxZeroDurationReturnsImmediately(t *testing.T) {
	if !SleepCtx(t.Context(), 0) {
		t.Errorf("zero duration should return true without waiting")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if SleepCtx(ctx, 0) {
		t.Errorf("zero duration on a canceled ctx should return false")
	}
}
