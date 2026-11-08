// Package repository — platform_day_queries.repo.go contains delivery/failure upserts and read queries for PlatformDayRepo.
package repository

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"quickbite/analytics-service/app/analytics/entity"
)

// UpsertOrderDelivered increments delivered_count, commission_sum, delivery_ms_sum, and delivery_ms_count.
func (r *PlatformDayRepo) UpsertOrderDelivered(
	ctx context.Context,
	date string,
	region string,
	currency string,
	commissionMinor int64,
	deliveryMs int64,
	now time.Time,
) error {
	reg := normalizeRegion(region)
	curr := normalizeCurrency(currency)
	filter := bson.M{
		"date":     date,
		"region":   reg,
		"currency": curr,
	}
	update := bson.M{
		"$inc": bson.M{
			"delivered_count":   int64(1),
			"commission_sum":    commissionMinor,
			"delivery_ms_sum":   deliveryMs,
			"delivery_ms_count": int64(1),
		},
		"$set": bson.M{
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"date":                 date,
			"region":               reg,
			"currency":             curr,
			"orders_count":         int64(0),
			"rejected_count":       int64(0),
			"cancelled_count":      int64(0),
			"gmv_sum":              int64(0),
			"service_fee_sum":      int64(0),
			"delivery_fee_sum":     int64(0),
			"online_payment_count": int64(0),
			"online_payment_sum":   int64(0),
			"payment_failed_count": int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}

// UpsertOrderStatusFailure increments rejected_count or cancelled_count on agg_platform_day.
func (r *PlatformDayRepo) UpsertOrderStatusFailure(
	ctx context.Context,
	date string,
	region string,
	currency string,
	isRejected bool,
	now time.Time,
) error {
	reg := normalizeRegion(region)
	curr := normalizeCurrency(currency)
	filter := bson.M{
		"date":     date,
		"region":   reg,
		"currency": curr,
	}
	incField := "cancelled_count"
	otherField := "rejected_count"
	if isRejected {
		incField = "rejected_count"
		otherField = "cancelled_count"
	}
	update := bson.M{
		"$inc": bson.M{
			incField: int64(1),
		},
		"$set": bson.M{
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"date":                 date,
			"region":               reg,
			"currency":             curr,
			"orders_count":         int64(0),
			"delivered_count":      int64(0),
			otherField:             int64(0),
			"gmv_sum":              int64(0),
			"commission_sum":       int64(0),
			"service_fee_sum":      int64(0),
			"delivery_fee_sum":     int64(0),
			"online_payment_count": int64(0),
			"online_payment_sum":   int64(0),
			"payment_failed_count": int64(0),
			"delivery_ms_sum":      int64(0),
			"delivery_ms_count":    int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}

// FindByDateRange returns platform daily rollups in [fromDate, toDate], optionally filtered by region.
// Backed by idx_date_region (date, region).
func (r *PlatformDayRepo) FindByDateRange(
	ctx context.Context,
	fromDate string,
	toDate string,
	region string,
) ([]entity.PlatformDayDoc, error) {
	filter := bson.M{
		"date": bson.M{
			"$gte": fromDate,
			"$lte": toDate,
		},
	}
	if region != "" && !strings.EqualFold(region, "all") {
		filter["region"] = strings.ToLower(strings.TrimSpace(region))
	}
	opts := options.Find().SetSort(bson.D{{Key: "date", Value: 1}, {Key: "region", Value: 1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []entity.PlatformDayDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// BulkWrite executes a batch of WriteModels in a single round-trip (used by cmd/backfill-aggs).
func (r *PlatformDayRepo) BulkWrite(ctx context.Context, models []mongo.WriteModel) error {
	if len(models) == 0 {
		return nil
	}
	_, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}

