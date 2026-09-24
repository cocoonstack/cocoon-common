// Package log provides shared log initialization for cocoonstack projects.
package log

import (
	"cmp"
	"context"
	"fmt"
	"os"

	corelog "github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Setup initializes the core logger from envVar (default "info"), JSON when stderr is not a terminal.
func Setup(ctx context.Context, envVar string) error {
	level := cmp.Or(os.Getenv(envVar), "info")
	cfg := &types.ServerLogConfig{Level: level, UseJSON: !stderrIsTerminal()}
	if err := corelog.SetupLog(ctx, cfg, ""); err != nil {
		return fmt.Errorf("setup log: %w", err)
	}
	return nil
}

func stderrIsTerminal() bool {
	fi, err := os.Stderr.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
