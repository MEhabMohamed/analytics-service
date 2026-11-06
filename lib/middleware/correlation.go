// Package middleware provides HTTP correlation ID propagation and structured access logging.
//
// TS vs Go Syntax Note:
//   - In Express, middleware has signature `(req, res, next) => { ...; next(); }`.
//   - In Go (`net/http` & Chi), middleware is a higher-order function `func(next http.Handler) http.Handler`
//     that returns an `http.HandlerFunc` calling `next.ServeHTTP(w, r)`.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"quickbite/analytics-service/lib/appcontext"
	"quickbite/analytics-service/lib/logger"
)

const CorrelationHeader = "X-CorrelationId"

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Correlation reads or generates X-CorrelationId and stores it in context and response headers.
func Correlation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := r.Header.Get(CorrelationHeader)
		if corrID == "" {
			corrID = r.Header.Get("X-Correlation-Id")
		}
		if corrID == "" {
			corrID = uuid.NewString()
		}
		w.Header().Set(CorrelationHeader, corrID)
		ctx := appcontext.WithCorrelationID(r.Context(), corrID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AccessLog logs HTTP method, path, status, and duration in structured JSON.
func AccessLog(baseLog *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log := logger.FromContext(r.Context(), baseLog)
			log.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"durationMs", time.Since(start).Milliseconds(),
			)
		})
	}
}

