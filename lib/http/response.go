// Package http provides canonical JSON response envelope helpers (`SendSuccess`, `SendPaginated`).
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/http/response.ts` (`sendSuccess`, `sendPaginated`).
//   - In Express, `res.status(statusCode).json(body)` sets the Content-Type header, status code, and serializes JSON.
//   - In Go's `net/http`, you must call `w.Header().Set("Content-Type", "application/json")` BEFORE
//     `w.WriteHeader(status)`, and then stream JSON with `json.NewEncoder(w).Encode(...)`.
package http

import (
	"encoding/json"
	"net/http"
)

type successEnvelope struct {
	Success bool `json:"success"`
	Data    any  `json:"data"`
	Meta    any  `json:"meta,omitempty"`
}

// SendSuccess writes {"success": true, "data": ...} with the given HTTP status code.
func SendSuccess(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(successEnvelope{
		Success: true,
		Data:    data,
	})
}

// SendPaginated writes {"success": true, "data": ..., "meta": ...} with HTTP 200 OK.
func SendPaginated(w http.ResponseWriter, data any, meta any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(successEnvelope{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

