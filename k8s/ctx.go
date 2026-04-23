package k8s

import (
	"context"
	"time"
)

// SleepCtx blocks for d or until ctx is canceled. Returns false if ctx fired first.
func SleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
