// Package dto defines Response DTO structs and mapper functions for HTTP endpoints.
//
// Divided into 2 cohesive files in `package dto`:
//   - analytics.response.go : Restaurant, branch, and product Response DTOs and mappers
//   - platform.response.go  : Platform-wide admin Response DTOs and mappers
//
// TS vs Go Syntax Note:
//   - In order-service (`order.response.dto.ts`), Response DTOs are classes with a static
//     factory method: `OrderResponseDTO.from(order, items)`.
//   - Go does not have classes or `static` methods. Instead, we define a plain `struct`
//     with `json:"camelCase"` struct tags (which control how `encoding/json` serializes
//     fields on the wire) paired with package-level mapper functions like
//     `FromRestaurantDayRows(rows)` (the Go equivalent of `Array.map(DTO.from)`).
//   - Important Go Slice Gotcha: An uninitialized slice in Go (`var out []DTO`) is `nil`
//     and serializes to JSON `null`, whereas an allocated empty slice (`make([]DTO, 0, len(rows))`)
//     serializes to JSON `[]`. Always use `make(..., 0, len(rows))` so empty results return `[]`!
package dto

import "quickbite/analytics-service/app/analytics"

// ─── 1. GET /restaurants/:restaurantId/days ──────────────────────────────────

// RestaurantDayResponseDTO is the wire shape for a single day in GET /restaurants/:restaurantId/days.
type RestaurantDayResponseDTO struct {
	Date          string `json:"date"`
	OrdersCount   int64  `json:"ordersCount"`
	RevenueMinor  int64  `json:"revenueMinor"`
	Currency      string `json:"currency"`
	AvgOrderMinor int64  `json:"avgOrderMinor"`
}

// FromRestaurantDayRows maps service rows to a non-nil slice of RestaurantDayResponseDTO.
func FromRestaurantDayRows(rows []analytics.RestaurantDayRow) []RestaurantDayResponseDTO {
	out := make([]RestaurantDayResponseDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, RestaurantDayResponseDTO{
			Date:          r.Date,
			OrdersCount:   r.OrdersCount,
			RevenueMinor:  r.RevenueMinor,
			Currency:      r.Currency,
			AvgOrderMinor: r.AvgOrderMinor,
		})
	}
	return out
}

// ─── 2. GET /restaurants/:restaurantId/failures ──────────────────────────────

type RestaurantFailureDayDTO struct {
	Date           string `json:"date"`
	OrdersCount    int64  `json:"ordersCount"`
	RejectedCount  int64  `json:"rejectedCount"`
	CancelledCount int64  `json:"cancelledCount"`
	FailedCount    int64  `json:"failedCount"`
	FailureRateBps int64  `json:"failureRateBps"`
}

type RestaurantFailuresResponseDTO struct {
	TotalOrders    int64                     `json:"totalOrders"`
	TotalRejected  int64                     `json:"totalRejected"`
	TotalCancelled int64                     `json:"totalCancelled"`
	TotalFailed    int64                     `json:"totalFailed"`
	FailureRateBps int64                     `json:"failureRateBps"`
	Days           []RestaurantFailureDayDTO `json:"days"`
}

func FromRestaurantFailuresSummary(s analytics.RestaurantFailuresSummary) RestaurantFailuresResponseDTO {
	days := make([]RestaurantFailureDayDTO, 0, len(s.Days))
	for _, d := range s.Days {
		days = append(days, RestaurantFailureDayDTO{
			Date:           d.Date,
			OrdersCount:    d.OrdersCount,
			RejectedCount:  d.RejectedCount,
			CancelledCount: d.CancelledCount,
			FailedCount:    d.FailedCount,
			FailureRateBps: d.FailureRateBps,
		})
	}
	return RestaurantFailuresResponseDTO{
		TotalOrders:    s.TotalOrders,
		TotalRejected:  s.TotalRejected,
		TotalCancelled: s.TotalCancelled,
		TotalFailed:    s.TotalFailed,
		FailureRateBps: s.FailureRateBps,
		Days:           days,
	}
}

// ─── 3. GET /restaurants/:restaurantId/delivery-avg ──────────────────────────

type RestaurantDeliveryAvgDayDTO struct {
	Date           string `json:"date"`
	DeliveredCount int64  `json:"deliveredCount"`
	AvgDeliveryMs  int64  `json:"avgDeliveryMs"`
	AvgDeliverySec int64  `json:"avgDeliverySec"`
}

type RestaurantDeliveryAvgResponseDTO struct {
	TotalDelivered int64                         `json:"totalDelivered"`
	AvgDeliveryMs  int64                         `json:"avgDeliveryMs"`
	AvgDeliverySec int64                         `json:"avgDeliverySec"`
	Days           []RestaurantDeliveryAvgDayDTO `json:"days"`
}

func FromRestaurantDeliveryAvgSummary(s analytics.RestaurantDeliveryAvgSummary) RestaurantDeliveryAvgResponseDTO {
	days := make([]RestaurantDeliveryAvgDayDTO, 0, len(s.Days))
	for _, d := range s.Days {
		days = append(days, RestaurantDeliveryAvgDayDTO{
			Date:           d.Date,
			DeliveredCount: d.DeliveredCount,
			AvgDeliveryMs:  d.AvgDeliveryMs,
			AvgDeliverySec: d.AvgDeliverySec,
		})
	}
	return RestaurantDeliveryAvgResponseDTO{
		TotalDelivered: s.TotalDelivered,
		AvgDeliveryMs:  s.AvgDeliveryMs,
		AvgDeliverySec: s.AvgDeliverySec,
		Days:           days,
	}
}

// ─── 4. GET /restaurants/:restaurantId/branches/days ─────────────────────────

type BranchDayResponseDTO struct {
	BranchID       int64  `json:"branchId"`
	RestaurantID   int64  `json:"restaurantId"`
	Date           string `json:"date"`
	Currency       string `json:"currency"`
	OrdersCount    int64  `json:"ordersCount"`
	DeliveredCount int64  `json:"deliveredCount"`
	RejectedCount  int64  `json:"rejectedCount"`
	CancelledCount int64  `json:"cancelledCount"`
	RevenueMinor   int64  `json:"revenueMinor"`
	AvgOrderMinor  int64  `json:"avgOrderMinor"`
	AvgDeliveryMs  int64  `json:"avgDeliveryMs"`
}

func FromBranchDayRows(rows []analytics.BranchDayRow) []BranchDayResponseDTO {
	out := make([]BranchDayResponseDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, BranchDayResponseDTO{
			BranchID:       r.BranchID,
			RestaurantID:   r.RestaurantID,
			Date:           r.Date,
			Currency:       r.Currency,
			OrdersCount:    r.OrdersCount,
			DeliveredCount: r.DeliveredCount,
			RejectedCount:  r.RejectedCount,
			CancelledCount: r.CancelledCount,
			RevenueMinor:   r.RevenueMinor,
			AvgOrderMinor:  r.AvgOrderMinor,
			AvgDeliveryMs:  r.AvgDeliveryMs,
		})
	}
	return out
}

// ─── 5. GET /restaurants/:restaurantId/products/top ──────────────────────────

type TopProductResponseDTO struct {
	ProductID    int64  `json:"productId"`
	QuantitySold int64  `json:"quantitySold"`
	RevenueMinor int64  `json:"revenueMinor"`
	OrdersCount  int64  `json:"ordersCount"`
	Currency     string `json:"currency"`
}

func FromTopProductRows(rows []analytics.TopProductRow) []TopProductResponseDTO {
	out := make([]TopProductResponseDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, TopProductResponseDTO{
			ProductID:    r.ProductID,
			QuantitySold: r.QuantitySold,
			RevenueMinor: r.RevenueMinor,
			OrdersCount:  r.OrdersCount,
			Currency:     r.Currency,
		})
	}
	return out
}
