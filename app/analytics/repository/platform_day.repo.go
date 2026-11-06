// Package repository implements MongoDB data access for agg_platform_day.
//
// Divided into 2 cohesive files in `package repository`:
//   - platform_day.repo.go         : PlatformDayRepo struct, constructor, and order/payment placement upserts
//   - platform_day_queries.repo.go : Delivery/failure upserts, FindByDateRange, and BulkWrite
//
// TS vs Go Syntax Note:
//   - Lowercase helper functions (`normalizeRegion`, `normalizeCurrency`) are package-private
//     in Go (equivalent to non-exported `function normalizeRegion(...)` in TypeScript) and are
//     accessible across all `.go` files in `package repository`.
package repository

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"quickbite/analytics-service/app/analytics"
)

// PlatformDayRepo manages the agg_platform_day collection.
type PlatformDayRepo struct {
	coll *mongo.Collection
}

// NewPlatformDayRepo constructs a PlatformDayRepo.
func NewPlatformDayRepo(db *mongo.Database) *PlatformDayRepo {
	return &PlatformDayRepo{
		coll: db.Collection(analytics.CollPlatformDay),
	}
}

func normalizeRegion(region string) string {
	r := strings.ToLower(strings.TrimSpace(region))
	if r == "" {
		return "eg"
	}
	return r
}

func normalizeCurrency(currency string) string {
	c := strings.ToUpper(strings.TrimSpace(currency))
	if c == "" {
		return "EGP"
	}
	return c
}

// UpsertOrderPlaced increments orders_count, gmv_sum, service_fee_sum, and delivery_fee_sum for a platform day.
func (r *PlatformDayRepo) UpsertOrderPlaced(
	ctx context.Context,
	date string,
	region string,
	currency string,
	totalMinor int64,
	serviceFeeMinor int64,
	deliveryFeeMinor int64,
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
			"orders_count":     int64(1),
			"gmv_sum":          totalMinor,
			"service_fee_sum":  serviceFeeMinor,
			"delivery_fee_sum": deliveryFeeMinor,
		},
		"$set": bson.M{
			"updated_at": now,
		},
		"$setOnInsert": bson.M{
			"date":                 date,
			"region":               reg,
			"currency":             curr,
			"delivered_count":      int64(0),
			"rejected_count":       int64(0),
			"cancelled_count":      int64(0),
			"commission_sum":       int64(0),
			"online_payment_count": int64(0),
			"online_payment_sum":   int64(0),
			"payment_failed_count": int64(0),
			"delivery_ms_sum":      int64(0),
			"delivery_ms_count":    int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// UpsertPaymentCompleted increments online_payment_count and online_payment_sum on agg_platform_day.
func (r *PlatformDayRepo) UpsertPaymentCompleted(
	ctx context.Context,
	date string,
	region string,
	currency string,
	amountMinor int64,
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
			"online_payment_count": int64(1),
			"online_payment_sum":   amountMinor,
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
			"rejected_count":       int64(0),
			"cancelled_count":      int64(0),
			"gmv_sum":              int64(0),
			"commission_sum":       int64(0),
			"service_fee_sum":      int64(0),
			"delivery_fee_sum":     int64(0),
			"payment_failed_count": int64(0),
			"delivery_ms_sum":      int64(0),
			"delivery_ms_count":    int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// UpsertPaymentFailed increments payment_failed_count on agg_platform_day.
func (r *PlatformDayRepo) UpsertPaymentFailed(
	ctx context.Context,
	date string,
	region string,
	currency string,
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
			"payment_failed_count": int64(1),
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
			"rejected_count":       int64(0),
			"cancelled_count":      int64(0),
			"gmv_sum":              int64(0),
			"commission_sum":       int64(0),
			"service_fee_sum":      int64(0),
			"delivery_fee_sum":     int64(0),
			"online_payment_count": int64(0),
			"online_payment_sum":   int64(0),
			"delivery_ms_sum":      int64(0),
			"delivery_ms_count":    int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}
