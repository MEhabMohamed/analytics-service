// Package service — restaurant.service.go contains restaurant, branch, and product read query projections.
package service

import (
	"context"

	"quickbite/analytics-service/app/analytics"
)

// GetRestaurantDays returns daily order/revenue rollups for a restaurant with derived AvgOrderMinor.
func (s *Service) GetRestaurantDays(ctx context.Context, q analytics.DateRangeQuery) ([]analytics.RestaurantDayRow, error) {
	if q.From > q.To {
		return nil, analytics.ErrInvalidDateRange
	}

	docs, err := s.restaurantRepo.FindByRestaurantAndDateRange(ctx, q.RestaurantID, q.From, q.To)
	if err != nil {
		return nil, err
	}

	rows := make([]analytics.RestaurantDayRow, 0, len(docs))
	for _, d := range docs {
		var avg int64
		if d.OrdersCount > 0 {
			avg = d.RevenueSum / d.OrdersCount
		}
		rows = append(rows, analytics.RestaurantDayRow{
			Date:          d.Date,
			OrdersCount:   d.OrdersCount,
			RevenueMinor:  d.RevenueSum,
			Currency:      normalizeCurrency(d.Currency),
			AvgOrderMinor: avg,
		})
	}
	return rows, nil
}

// GetRestaurantFailures derives daily and overall failure rates from agg_restaurant_day (no extra collection needed).
func (s *Service) GetRestaurantFailures(ctx context.Context, q analytics.DateRangeQuery) (analytics.RestaurantFailuresSummary, error) {
	if q.From > q.To {
		return analytics.RestaurantFailuresSummary{}, analytics.ErrInvalidDateRange
	}

	docs, err := s.restaurantRepo.FindByRestaurantAndDateRange(ctx, q.RestaurantID, q.From, q.To)
	if err != nil {
		return analytics.RestaurantFailuresSummary{}, err
	}

	var summary analytics.RestaurantFailuresSummary
	summary.Days = make([]analytics.RestaurantFailureDayRow, 0, len(docs))

	for _, d := range docs {
		failed := d.RejectedCount + d.CancelledCount
		var rateBps int64
		if d.OrdersCount > 0 {
			rateBps = (failed * 10000) / d.OrdersCount
		}
		summary.TotalOrders += d.OrdersCount
		summary.TotalRejected += d.RejectedCount
		summary.TotalCancelled += d.CancelledCount
		summary.TotalFailed += failed
		summary.Days = append(summary.Days, analytics.RestaurantFailureDayRow{
			Date:           d.Date,
			OrdersCount:    d.OrdersCount,
			RejectedCount:  d.RejectedCount,
			CancelledCount: d.CancelledCount,
			FailedCount:    failed,
			FailureRateBps: rateBps,
		})
	}

	if summary.TotalOrders > 0 {
		summary.FailureRateBps = (summary.TotalFailed * 10000) / summary.TotalOrders
	}
	return summary, nil
}

// GetRestaurantDeliveryAvg derives daily and overall average delivery times from agg_restaurant_day (delivery_ms_sum / delivery_ms_count).
func (s *Service) GetRestaurantDeliveryAvg(ctx context.Context, q analytics.DateRangeQuery) (analytics.RestaurantDeliveryAvgSummary, error) {
	if q.From > q.To {
		return analytics.RestaurantDeliveryAvgSummary{}, analytics.ErrInvalidDateRange
	}

	docs, err := s.restaurantRepo.FindByRestaurantAndDateRange(ctx, q.RestaurantID, q.From, q.To)
	if err != nil {
		return analytics.RestaurantDeliveryAvgSummary{}, err
	}

	var summary analytics.RestaurantDeliveryAvgSummary
	summary.Days = make([]analytics.RestaurantDeliveryAvgDayRow, 0, len(docs))

	var totalMsSum int64
	var totalMsCount int64

	for _, d := range docs {
		var avgMs int64
		if d.DeliveryMsCount > 0 {
			avgMs = d.DeliveryMsSum / d.DeliveryMsCount
		}
		totalMsSum += d.DeliveryMsSum
		totalMsCount += d.DeliveryMsCount
		summary.TotalDelivered += d.DeliveredCount

		summary.Days = append(summary.Days, analytics.RestaurantDeliveryAvgDayRow{
			Date:           d.Date,
			DeliveredCount: d.DeliveredCount,
			AvgDeliveryMs:  avgMs,
			AvgDeliverySec: avgMs / 1000,
		})
	}

	if totalMsCount > 0 {
		summary.AvgDeliveryMs = totalMsSum / totalMsCount
		summary.AvgDeliverySec = summary.AvgDeliveryMs / 1000
	}
	return summary, nil
}

// GetBranchDays returns daily rollups per branch for a restaurant (or a single branch) with derived averages.
func (s *Service) GetBranchDays(ctx context.Context, q analytics.DateRangeQuery) ([]analytics.BranchDayRow, error) {
	if q.From > q.To {
		return nil, analytics.ErrInvalidDateRange
	}

	docs, err := s.branchRepo.FindByRestaurantAndDateRange(ctx, q.RestaurantID, q.BranchID, q.From, q.To)
	if err != nil {
		return nil, err
	}

	rows := make([]analytics.BranchDayRow, 0, len(docs))
	for _, d := range docs {
		var avgOrder int64
		if d.OrdersCount > 0 {
			avgOrder = d.RevenueSum / d.OrdersCount
		}
		var avgDelivMs int64
		if d.DeliveryMsCount > 0 {
			avgDelivMs = d.DeliveryMsSum / d.DeliveryMsCount
		}
		rows = append(rows, analytics.BranchDayRow{
			BranchID:       d.BranchID,
			RestaurantID:   d.RestaurantID,
			Date:           d.Date,
			Currency:       normalizeCurrency(d.Currency),
			OrdersCount:    d.OrdersCount,
			DeliveredCount: d.DeliveredCount,
			RejectedCount:  d.RejectedCount,
			CancelledCount: d.CancelledCount,
			RevenueMinor:   d.RevenueSum,
			AvgOrderMinor:  avgOrder,
			AvgDeliveryMs:  avgDelivMs,
		})
	}
	return rows, nil
}

// GetTopProducts returns the top N products by revenue across [from, to] for a restaurant (or branch).
func (s *Service) GetTopProducts(ctx context.Context, q analytics.DateRangeQuery) ([]analytics.TopProductRow, error) {
	if q.From > q.To {
		return nil, analytics.ErrInvalidDateRange
	}
	return s.productRepo.FindTopProducts(ctx, q.RestaurantID, q.BranchID, q.From, q.To, q.Limit)
}

