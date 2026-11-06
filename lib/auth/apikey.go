// Package auth provides the shared-secret internal API key middleware.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/auth/api-key.ts` (`requireInternalApiKey`).
//   - Uses `crypto/subtle.ConstantTimeCompare` so secret comparison takes constant time regardless
//     of how many prefix bytes match.
package auth

import (
	"crypto/subtle"
	"log/slog"
	"net/http"

	apperr "quickbite/analytics-service/lib/errors"
)

// RequireInternalAPIKey guards internal service-to-service routes using the shared api-key header.
func RequireInternalAPIKey(expectedKey string, baseLog *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			provided := r.Header.Get("api-key")
			if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(expectedKey)) != 1 {
				apperr.RenderError(w, r, baseLog, apperr.ErrUnauthenticated.WithMessage("Invalid or missing api-key"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

