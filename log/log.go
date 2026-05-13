// Package log provides shared log initialization for cocoonstack projects.
package log

import (
	"context"
	"fmt"
	"os"

	corelog "github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Setup initializes the core logger from envVar (default "info").
//
// Returns the underlying setup error so the caller chooses the failure
// policy: a main package can Fatalf with its own logger, while a test
// or library caller can surface the error normally. The previous
// signature swallowed the error inside a Fatalf, which was unfriendly
// to callers that needed clean teardown.
func Setup(ctx context.Context, envVar string) error {
	level := os.Getenv(envVar)
	if level == "" {
		level = "info"
	}
	if err := corelog.SetupLog(ctx, &types.ServerLogConfig{Level: level}, ""); err != nil {
		return fmt.Errorf("setup log: %w", err)
	}
	return nil
}
