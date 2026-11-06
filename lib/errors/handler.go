// Package errors provides the Wrap adapter that converts error-returning handlers into http.HandlerFunc.
//
// TS vs Go Syntax Note:
//   - Express v5 automatically catches rejected Promises from `async (req, res)` handlers and forwards
//     them to the 4-argument `errorHandler(err, req, res, next)` middleware.
//   - Go's standard `http.HandlerFunc` signature is `func(w http.ResponseWriter, r *http.Request)` (no return value).
//     By defining `type HandlerFunc func(w http.ResponseWriter, r *http.Request) error` and wrapping routes with
//     `errors.Wrap(log, ctrl.Method)`, controllers can simply `return err` and let `RenderError` format the JSON envelope.
package errors

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"quickbite/analytics-service/lib/logger"
)

// HandlerFunc is an HTTP handler that returns an error rather than writing error responses directly.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Success bool        `json:"success"`
	Error   errorDetail `json:"error"`
}

// Wrap converts a HandlerFunc returning error into a standard http.HandlerFunc and renders the canonical error envelope.
func Wrap(baseLog *slog.Logger, h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := h(w, r)
		if err == nil {
			return
		}
		RenderError(w, r, baseLog, err)
	}
}

// RenderError writes the canonical {"success": false, "error": {"code": "...", "message": "..."}} envelope.
func RenderError(w http.ResponseWriter, r *http.Request, baseLog *slog.Logger, err error) {
	log := logger.FromContext(r.Context(), baseLog)

	var appErr *AppError
	if errors.As(err, &appErr) {
		log.Warn("request operational error",
			"code", appErr.Code,
			"status", appErr.Status,
			"message", appErr.Message,
			"path", r.URL.Path,
			"method", r.Method,
		)
		writeJSON(w, appErr.Status, errorEnvelope{
			Success: false,
			Error: errorDetail{
				Code:    appErr.Code,
				Message: appErr.Message,
			},
		})
		return
	}

	log.Error("unhandled request error",
		"error", err.Error(),
		"path", r.URL.Path,
		"method", r.Method,
	)
	writeJSON(w, ErrInternal.Status, errorEnvelope{
		Success: false,
		Error: errorDetail{
			Code:    ErrInternal.Code,
			Message: ErrInternal.Message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

