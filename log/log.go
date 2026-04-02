// Package log provides a shared log initialization function for cocoonstack projects.
package log

import (
	"context"
	"os"

	corelog "github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Setup initializes the core logger with the level from the named
// environment variable, defaulting to "info". Fatals on failure.
func Setup(ctx context.Context, envVar string) {
	level := os.Getenv(envVar)
	if level == "" {
		level = "info"
	}
	if err := corelog.SetupLog(ctx, &types.ServerLogConfig{Level: level}, ""); err != nil {
		corelog.WithFunc("cocooncommon.log.Setup").Fatalf(ctx, err, "setup log")
	}
}
