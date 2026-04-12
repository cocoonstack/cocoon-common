// Package log provides shared log initialization for cocoonstack projects.
package log

import (
	"context"
	"os"

	corelog "github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Setup initializes the core logger from envVar (default "info"). Fatals on failure.
func Setup(ctx context.Context, envVar string) {
	level := os.Getenv(envVar)
	if level == "" {
		level = "info"
	}
	if err := corelog.SetupLog(ctx, &types.ServerLogConfig{Level: level}, ""); err != nil {
		corelog.WithFunc("cocooncommon.log.Setup").Fatalf(ctx, err, "setup log: %v", err)
	}
}
