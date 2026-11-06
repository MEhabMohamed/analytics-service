// Package service — events.service.go contains all inbound RabbitMQ event write projections on *Service.
//
// TS vs Go Syntax Note:
//   - Even though `type Service struct` is declared in `analytics.service.go`, Go allows us to attach
//     methods (`func (s *Service) OnOrderPlaced(...)`) in a separate file within the same `package service`.
//   - Package-private helpers like `extractUTCDate` and `normalizeCurrency` defined in `analytics.service.go`
//     are directly callable here without any relative import.
package service

import (
	"context"
	"time"

	"quickbite/analytics-service/app/analytics"
)

// OnOrderPlaced updates all 4 daily aggregate collections for a newly placed order:
//   1. agg_restaurant_day ($inc orders_count, revenue_sum, cod stats)
//   2. agg_branch_day     ($inc orders_count, revenue_sum)
//   3. agg_product_day    (single BulkWrite across all line items — zero N+1)
//   4. agg_platform_day   ($inc orders_count, gmv_sum, service_fee_sum, delivery_fee_sum)
func (s *Service) OnOrderPlaced(ctx context.Context, in analytics.OnOrderPlacedInput) error {
	date := extractUTCDate(in.PlacedAt)
	now := time.Now().UTC()
	currency := normalizeCurrency(in.Currency)
	region := firstNonEmpty(in.Region, in.CountryCode, "eg")

	if err := s.restaurantRepo.UpsertOrderPlaced(
		ctx,
		in.RestaurantID,
		date,
		currency,
		in.Total,
		in.PaymentMethod,
		now,
	); err != nil {
		return err
	}

	if in.BranchID > 0 {
		if err := s.branchRepo.UpsertOrderPlaced(
			ctx,
			in.BranchID,
			in.RestaurantID,
			date,
			currency,
			in.Total,
			now,
		); err != nil {
			return err
		}
	}

	if len(in.Items) > 0 && in.BranchID > 0 {
		if err := s.productRepo.BulkUpsertOrderItems(
			ctx,
			in.RestaurantID,
			in.BranchID,
			date,
			currency,
			in.Items,
			now,
		); err != nil {
			return err
		}
	}

	return s.platformRepo.UpsertOrderPlaced(
		ctx,
		date,
		region,
		currency,
		in.Total,
		in.ServiceFee,
		in.DeliveryFee,
		now,
	)
}

// OnPaymentCompleted increments online payment counters on agg_restaurant_day and agg_platform_day.
func (s *Service) OnPaymentCompleted(ctx context.Context, in analytics.OnPaymentCompletedInput) error {
	date := extractUTCDate(in.CompletedAt)
	now := time.Now().UTC()
	currency := normalizeCurrency(in.Currency)
	region := firstNonEmpty(in.Region, "eg")

	if in.RestaurantID > 0 {
		if err := s.restaurantRepo.UpsertPaymentCompleted(
			ctx,
			in.RestaurantID,
			date,
			currency,
			in.Amount,
			now,
		); err != nil {
			return err
		}
	}

	return s.platformRepo.UpsertPaymentCompleted(
		ctx,
		date,
		region,
		currency,
		in.Amount,
		now,
	)
}

// OnPaymentFailed increments payment_failed_count on agg_platform_day.
func (s *Service) OnPaymentFailed(ctx context.Context, in analytics.OnPaymentFailedInput) error {
	date := extractUTCDate(in.FailedAt)
	now := time.Now().UTC()
	currency := normalizeCurrency(in.Currency)
	region := firstNonEmpty(in.Region, "eg")

	return s.platformRepo.UpsertPaymentFailed(ctx, date, region, currency, now)
}

// OnOrderDelivered updates delivery counts, associative delivery duration (delivery_ms_sum + delivery_ms_count),
// and platform commission across agg_restaurant_day, agg_branch_day, and agg_platform_day.
//
// Out-of-order gotcha handled:
//   - Buckets by PlacedAt (falling back to DeliveredAt) so the delivery stats land on the order's placement day.
//   - Uses $inc + $setOnInsert with Upsert=true so if order.delivered arrives BEFORE order.placed,
//     both events commute associatively without losing either count or revenue.
func (s *Service) OnOrderDelivered(ctx context.Context, in analytics.OnOrderDeliveredInput) error {
	date := extractUTCDate(firstNonEmpty(in.PlacedAt, in.DeliveredAt))
	deliveryMs := computeDeliveryMs(in.PlacedAt, in.DeliveredAt)
	now := time.Now().UTC()
	currency := normalizeCurrency(in.Currency)
	region := firstNonEmpty(in.Region, in.CountryCode, "eg")

	if err := s.restaurantRepo.UpsertOrderDelivered(
		ctx,
		in.RestaurantID,
		date,
		currency,
		deliveryMs,
		now,
	); err != nil {
		return err
	}

	if in.BranchID > 0 {
		if err := s.branchRepo.UpsertOrderDelivered(
			ctx,
			in.BranchID,
			in.RestaurantID,
			date,
			currency,
			deliveryMs,
			now,
		); err != nil {
			return err
		}
	}

	return s.platformRepo.UpsertOrderDelivered(
		ctx,
		date,
		region,
		currency,
		in.Commission,
		deliveryMs,
		now,
	)
}

// OnOrderRejected increments rejected_count on agg_restaurant_day, agg_branch_day, and agg_platform_day.
func (s *Service) OnOrderRejected(ctx context.Context, in analytics.OnOrderTransitionInput) error {
	return s.recordOrderFailure(ctx, in, true)
}

// OnOrderCancelled increments cancelled_count on agg_restaurant_day, agg_branch_day, and agg_platform_day.
func (s *Service) OnOrderCancelled(ctx context.Context, in analytics.OnOrderTransitionInput) error {
	return s.recordOrderFailure(ctx, in, false)
}

func (s *Service) recordOrderFailure(ctx context.Context, in analytics.OnOrderTransitionInput, isRejected bool) error {
	date := extractUTCDate(firstNonEmpty(in.PlacedAt, in.OccurredAt))
	now := time.Now().UTC()
	currency := normalizeCurrency(in.Currency)
	region := firstNonEmpty(in.Region, in.CountryCode, "eg")

	if err := s.restaurantRepo.UpsertOrderStatusFailure(
		ctx,
		in.RestaurantID,
		date,
		currency,
		isRejected,
		now,
	); err != nil {
		return err
	}

	if in.BranchID > 0 {
		if err := s.branchRepo.UpsertOrderStatusFailure(
			ctx,
			in.BranchID,
			in.RestaurantID,
			date,
			currency,
			isRejected,
			now,
		); err != nil {
			return err
		}
	}

	return s.platformRepo.UpsertOrderStatusFailure(
		ctx,
		date,
		region,
		currency,
		isRejected,
		now,
	)
}

