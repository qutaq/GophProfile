package observability

import (
	"context"
	"log/slog"
)

type loggerCtxKey struct{}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, logger)
}

func LoggerFromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}
