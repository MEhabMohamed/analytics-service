// Package logger wraps Go's standard library structured logger (`log/slog`).
//
// TS vs Go Syntax Note:
//   - In `order-service/src/lib/logger/logger.ts`, a custom `Logger` class calls `JSON.stringify(logObject)`.
//   - Since Go 1.21, the standard library includes `log/slog` with a built-in `JSONHandler` and
//     key-value alternating arguments (`log.Info("msg", "key1", val1, "key2", val2)`).
package logger

import (
	"context"
	"log/slog"
	"os"

	"quickbite/analytics-service/lib/appcontext"
)

// New creates a structured JSON slog.Logger writing to stdout.
func New() *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}

// FromContext returns a child logger enriched with the request's correlationId if present.
func FromContext(ctx context.Context, base *slog.Logger) *slog.Logger {
	if base == nil {
		base = New()
	}
	if corrID := appcontext.CorrelationIDFromContext(ctx); corrID != "" {
		return base.With("correlationId", corrID)
	}
	return base
}

