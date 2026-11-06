// Package dto holds the wire-format Request and Response structures for the analytics module.
//
// TS vs Go Syntax Note:
//   - In TypeScript (order-service), request validation uses class-validator decorators
//     like `@IsString() @Matches(/^\d{4}-\d{2}-\d{2}$/) from!: string;` on a class.
//   - Go does not have runtime decorators (`@Decorator`). Instead, Go attaches metadata
//     to struct fields using backtick "struct tags" (`validate:"required,datetime=2006-01-02"`)
//     which are inspected via reflection by `github.com/go-playground/validator/v10`.
//   - Notice Go's reference date layout `2006-01-02`: Go formats/parses dates using a
//     mnemonic reference timestamp (Jan 2, 15:04:05 2006 MST = 01 02 03 04 05 06 -07)
//     rather than `YYYY-MM-DD`.
package dto

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"

	"quickbite/analytics-service/app/analytics"
)

// Shared validator singleton (thread-safe and caches struct reflection metadata).
// In Node/TS, `validateBody(DTO, req.body)` instantiates class-validator per call;
// in Go, `validator.New()` is created once at package load so struct tag parsing is cached.
var validate = validator.New()

// RestaurantDateRangeRequest validates path `restaurantId` and query `from`, `to`.
//
// TS vs Go Syntax Note:
//   - Exported fields MUST start with an Uppercase letter (`RestaurantID`, `From`, `To`).
//     In Go, capitalization controls visibility (`public` vs `private`), replacing TS's
//     `public` / `private` keywords. If a field is lowercase, `validator` and `encoding/json`
//     cannot see it!
type RestaurantDateRangeRequest struct {
	RestaurantID int64  `validate:"required,gt=0"`
	BranchID     int64  `validate:"omitempty,gt=0"`
	From         string `validate:"required,datetime=2006-01-02"`
	To           string `validate:"required,datetime=2006-01-02"`
	Limit        int64  `validate:"omitempty,gt=0,lte=100"`
}

// PlatformDateRangeRequest validates query `from`, `to`, and optional `region` for admin endpoints.
type PlatformDateRangeRequest struct {
	From   string `validate:"required,datetime=2006-01-02"`
	To     string `validate:"required,datetime=2006-01-02"`
	Region string `validate:"omitempty"`
}

// ParseRestaurantDateRange extracts path/query parameters from *http.Request and validates them.
//
// TS vs Go Syntax Note:
//   - In Express (order-service), you write `Number(req.params.restaurantId)` and `req.query.from`,
//     and throw `AppError` on failure.
//   - In Go, functions do not `throw` exceptions; they return `(T, error)` as a multi-value tuple.
//     Callers check `if err != nil { return err }`.
func ParseRestaurantDateRange(r *http.Request) (analytics.DateRangeQuery, error) {
	rawRestID := chi.URLParam(r, "restaurantId")
	restID, err := strconv.ParseInt(rawRestID, 10, 64)
	if err != nil || restID <= 0 {
		return analytics.DateRangeQuery{}, analytics.ErrValidation.WithMessage("restaurantId must be a positive integer")
	}

	var branchID int64
	if rawBranch := strings.TrimSpace(r.URL.Query().Get("branchId")); rawBranch != "" {
		bID, bErr := strconv.ParseInt(rawBranch, 10, 64)
		if bErr != nil || bID <= 0 {
			return analytics.DateRangeQuery{}, analytics.ErrValidation.WithMessage("branchId must be a positive integer")
		}
		branchID = bID
	}

	var limit int64 = 10
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		l, lErr := strconv.ParseInt(rawLimit, 10, 64)
		if lErr != nil || l <= 0 || l > 100 {
			return analytics.DateRangeQuery{}, analytics.ErrValidation.WithMessage("limit must be between 1 and 100")
		}
		limit = l
	}

	req := RestaurantDateRangeRequest{
		RestaurantID: restID,
		BranchID:     branchID,
		From:         strings.TrimSpace(r.URL.Query().Get("from")),
		To:           strings.TrimSpace(r.URL.Query().Get("to")),
		Limit:        limit,
	}

	if err := validate.Struct(req); err != nil {
		return analytics.DateRangeQuery{}, analytics.ErrValidation.WithMessage("Query parameters 'from' and 'to' are required in YYYY-MM-DD format")
	}

	return analytics.DateRangeQuery{
		RestaurantID: req.RestaurantID,
		BranchID:     req.BranchID,
		From:         req.From,
		To:           req.To,
		Limit:        req.Limit,
	}, nil
}

// ParsePlatformDateRange extracts and validates query parameters for platform-wide admin routes.
func ParsePlatformDateRange(r *http.Request) (analytics.DateRangeQuery, error) {
	req := PlatformDateRangeRequest{
		From:   strings.TrimSpace(r.URL.Query().Get("from")),
		To:     strings.TrimSpace(r.URL.Query().Get("to")),
		Region: strings.TrimSpace(r.URL.Query().Get("region")),
	}

	if err := validate.Struct(req); err != nil {
		return analytics.DateRangeQuery{}, analytics.ErrValidation.WithMessage("Query parameters 'from' and 'to' are required in YYYY-MM-DD format")
	}

	return analytics.DateRangeQuery{
		From:   req.From,
		To:     req.To,
		Region: req.Region,
	}, nil
}

