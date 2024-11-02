package logging

import (
	"context"
	"log/slog"
)

func Context(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, "loggerKey", logger)
}

func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value("loggerKey").(*slog.Logger); ok {
		return logger
	}

	return slog.Default()
}
