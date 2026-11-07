package safe

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
)

// Go starts a goroutine that cannot take down the process with a panic.
func Go(ctx context.Context, logger *slog.Logger, name string, fn func(context.Context)) {
	go func() {
		defer Recover(logger, name)
		fn(ctx)
	}()
}

// Recover logs a recovered panic with its stack. Use as a defer at a boundary.
func Recover(logger *slog.Logger, name string) {
	if recovered := recover(); recovered != nil {
		logger.Error("recovered from panic",
			"boundary", name,
			"panic", fmt.Sprint(recovered),
			"stack", string(debug.Stack()),
		)
	}
}
