// Package service — platform.service.go contains platform-wide admin read query projections.
package service

import (
	"context"

	"quickbite/analytics-service/app/analytics"
)

// GetPlatformDays returns daily platform-wide rollups with derived AvgOrderMinor and AvgDeliveryMs.
func (s *Service) GetPlatformDays(ctx context.Context, q analytics.DateRangeQuery) ([]analytics.PlatformDayRow, error) {
	if q.From > q.To {
		return nil, analytics.ErrInvalidDateRange
	}

	docs, err := s.platformRepo.FindByDateRange(ctx, q.From, q.To, q.Region)
	if err != nil {
		return nil, err
	}

	rows := make([]analytics.PlatformDayRow, 0, len(docs))
	for _, d := range docs {
		var avgOrder int64
		if d.OrdersCount > 0 {
			avgOrder = d.GMVSum / d.OrdersCount
		}
		var avgDelivMs int64
		if d.DeliveryMsCount > 0 {
			avgDelivMs = d.DeliveryMsSum / d.DeliveryMsCount
		}
		rows = append(rows, analytics.PlatformDayRow{
			Date:               d.Date,
			Region:             d.Region,
			Currency:           normalizeCurrency(d.Currency),
			OrdersCount:        d.OrdersCount,
			DeliveredCount:     d.DeliveredCount,
			RejectedCount:      d.RejectedCount,
			CancelledCount:     d.CancelledCount,
			GMVMinor:           d.GMVSum,
			CommissionMinor:    d.CommissionSum,
			ServiceFeeMinor:    d.ServiceFeeSum,
			DeliveryFeeMinor:   d.DeliveryFeeSum,
			OnlinePaymentCount: d.OnlinePaymentCount,
			OnlinePaymentMinor: d.OnlinePaymentSum,
			PaymentFailedCount: d.PaymentFailedCount,
			AvgOrderMinor:      avgOrder,
			AvgDeliveryMs:      avgDelivMs,
		})
	}
	return rows, nil
}

// GetActiveRestaurants derives daily and range-total active restaurants from agg_restaurant_day.
func (s *Service) GetActiveRestaurants(ctx context.Context, q analytics.DateRangeQuery) (analytics.ActiveRestaurantsSummary, error) {
	if q.From > q.To {
		return analytics.ActiveRestaurantsSummary{}, analytics.ErrInvalidDateRange
	}
	total, days, err := s.restaurantRepo.AggregateActiveRestaurants(ctx, q.From, q.To)
	if err != nil {
		return analytics.ActiveRestaurantsSummary{}, err
	}
	return analytics.ActiveRestaurantsSummary{
		TotalActiveRestaurants: total,
		Days:                   days,
	}, nil
}

// GetPlatformFailures derives daily and range-total order & payment failure metrics from agg_platform_day.
func (s *Service) GetPlatformFailures(ctx context.Context, q analytics.DateRangeQuery) (analytics.PlatformFailuresSummary, error) {
	if q.From > q.To {
		return analytics.PlatformFailuresSummary{}, analytics.ErrInvalidDateRange
	}

	docs, err := s.platformRepo.FindByDateRange(ctx, q.From, q.To, q.Region)
	if err != nil {
		return analytics.PlatformFailuresSummary{}, err
	}

	var summary analytics.PlatformFailuresSummary
	summary.Days = make([]analytics.PlatformFailureDayRow, 0, len(docs))

	for _, d := range docs {
		failedOrders := d.RejectedCount + d.CancelledCount
		var rateBps int64
		if d.OrdersCount > 0 {
			rateBps = (failedOrders * 10000) / d.OrdersCount
		}
		summary.TotalOrders += d.OrdersCount
		summary.TotalRejected += d.RejectedCount
		summary.TotalCancelled += d.CancelledCount
		summary.TotalPaymentFailed += d.PaymentFailedCount

		summary.Days = append(summary.Days, analytics.PlatformFailureDayRow{
			Date:               d.Date,
			Region:             d.Region,
			OrdersCount:        d.OrdersCount,
			RejectedCount:      d.RejectedCount,
			CancelledCount:     d.CancelledCount,
			PaymentFailedCount: d.PaymentFailedCount,
			OrderFailureBps:    rateBps,
		})
	}

	totalFailedOrders := summary.TotalRejected + summary.TotalCancelled
	if summary.TotalOrders > 0 {
		summary.OrderFailureBps = (totalFailedOrders * 10000) / summary.TotalOrders
	}
	return summary, nil
}

