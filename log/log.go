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
