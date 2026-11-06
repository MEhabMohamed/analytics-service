// Package dto — platform.response.go defines Response DTO structs and mappers for platform-wide admin endpoints.
package dto

import "quickbite/analytics-service/app/analytics"

// ─── 6. GET /platform/days ───────────────────────────────────────────────────

type PlatformDayResponseDTO struct {
	Date               string `json:"date"`
	Region             string `json:"region"`
	Currency           string `json:"currency"`
	OrdersCount        int64  `json:"ordersCount"`
	DeliveredCount     int64  `json:"deliveredCount"`
	RejectedCount      int64  `json:"rejectedCount"`
	CancelledCount     int64  `json:"cancelledCount"`
	GMVMinor           int64  `json:"gmvMinor"`
	CommissionMinor    int64  `json:"commissionMinor"`
	ServiceFeeMinor    int64  `json:"serviceFeeMinor"`
	DeliveryFeeMinor   int64  `json:"deliveryFeeMinor"`
	OnlinePaymentCount int64  `json:"onlinePaymentCount"`
	OnlinePaymentMinor int64  `json:"onlinePaymentMinor"`
	PaymentFailedCount int64  `json:"paymentFailedCount"`
	AvgOrderMinor      int64  `json:"avgOrderMinor"`
	AvgDeliveryMs      int64  `json:"avgDeliveryMs"`
}

func FromPlatformDayRows(rows []analytics.PlatformDayRow) []PlatformDayResponseDTO {
	out := make([]PlatformDayResponseDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, PlatformDayResponseDTO{
			Date:               r.Date,
			Region:             r.Region,
			Currency:           r.Currency,
			OrdersCount:        r.OrdersCount,
			DeliveredCount:     r.DeliveredCount,
			RejectedCount:      r.RejectedCount,
			CancelledCount:     r.CancelledCount,
			GMVMinor:           r.GMVMinor,
			CommissionMinor:    r.CommissionMinor,
			ServiceFeeMinor:    r.ServiceFeeMinor,
			DeliveryFeeMinor:   r.DeliveryFeeMinor,
			OnlinePaymentCount: r.OnlinePaymentCount,
			OnlinePaymentMinor: r.OnlinePaymentMinor,
			PaymentFailedCount: r.PaymentFailedCount,
			AvgOrderMinor:      r.AvgOrderMinor,
			AvgDeliveryMs:      r.AvgDeliveryMs,
		})
	}
	return out
}

// ─── 7. GET /platform/active-restaurants ─────────────────────────────────────

type ActiveRestaurantsDayDTO struct {
	Date                   string `json:"date"`
	ActiveRestaurantsCount int64  `json:"activeRestaurantsCount"`
	OrdersCount            int64  `json:"ordersCount"`
	RevenueMinor           int64  `json:"revenueMinor"`
}

type ActiveRestaurantsResponseDTO struct {
	TotalActiveRestaurants int64                     `json:"totalActiveRestaurants"`
	Days                   []ActiveRestaurantsDayDTO `json:"days"`
}

func FromActiveRestaurantsSummary(s analytics.ActiveRestaurantsSummary) ActiveRestaurantsResponseDTO {
	days := make([]ActiveRestaurantsDayDTO, 0, len(s.Days))
	for _, d := range s.Days {
		days = append(days, ActiveRestaurantsDayDTO{
			Date:                   d.Date,
			ActiveRestaurantsCount: d.ActiveRestaurantsCount,
			OrdersCount:            d.OrdersCount,
			RevenueMinor:           d.RevenueMinor,
		})
	}
	return ActiveRestaurantsResponseDTO{
		TotalActiveRestaurants: s.TotalActiveRestaurants,
		Days:                   days,
	}
}

// ─── 8. GET /platform/failures ───────────────────────────────────────────────

type PlatformFailureDayDTO struct {
	Date               string `json:"date"`
	Region             string `json:"region"`
	OrdersCount        int64  `json:"ordersCount"`
	RejectedCount      int64  `json:"rejectedCount"`
	CancelledCount     int64  `json:"cancelledCount"`
	PaymentFailedCount int64  `json:"paymentFailedCount"`
	OrderFailureBps    int64  `json:"orderFailureBps"`
}

type PlatformFailuresResponseDTO struct {
	TotalOrders        int64                   `json:"totalOrders"`
	TotalRejected      int64                   `json:"totalRejected"`
	TotalCancelled     int64                   `json:"totalCancelled"`
	TotalPaymentFailed int64                   `json:"totalPaymentFailed"`
	OrderFailureBps    int64                   `json:"orderFailureBps"`
	Days               []PlatformFailureDayDTO `json:"days"`
}

func FromPlatformFailuresSummary(s analytics.PlatformFailuresSummary) PlatformFailuresResponseDTO {
	days := make([]PlatformFailureDayDTO, 0, len(s.Days))
	for _, d := range s.Days {
		days = append(days, PlatformFailureDayDTO{
			Date:               d.Date,
			Region:             d.Region,
			OrdersCount:        d.OrdersCount,
			RejectedCount:      d.RejectedCount,
			CancelledCount:     d.CancelledCount,
			PaymentFailedCount: d.PaymentFailedCount,
			OrderFailureBps:    d.OrderFailureBps,
		})
	}
	return PlatformFailuresResponseDTO{
		TotalOrders:        s.TotalOrders,
		TotalRejected:      s.TotalRejected,
		TotalCancelled:     s.TotalCancelled,
		TotalPaymentFailed: s.TotalPaymentFailed,
		OrderFailureBps:    s.OrderFailureBps,
		Days:               days,
	}
}

