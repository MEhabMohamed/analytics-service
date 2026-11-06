// Package repository implements MongoDB data access for agg_branch_day.
//
// TS vs Go Syntax Note:
//   - Notice `defer cur.Close(ctx)` in `FindByRestaurantAndDateRange`: Go's `defer` statement
//     schedules a function call to run immediately before the surrounding function returns
//     (similar to `try { ... } finally { await cursor.close(); }` in TypeScript, but written
//     right next to the resource acquisition so it can never be forgotten).
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"quickbite/analytics-service/app/analytics"
	"quickbite/analytics-service/app/analytics/entity"
)

// BranchDayRepo manages the agg_branch_day collection.
type BranchDayRepo struct {
	coll *mongo.Collection
}

// NewBranchDayRepo constructs a BranchDayRepo.
func NewBranchDayRepo(db *mongo.Database) *BranchDayRepo {
	return &BranchDayRepo{
		coll: db.Collection(analytics.CollBranchDay),
	}
}

// UpsertOrderPlaced increments orders_count and revenue_sum for a branch day.
func (r *BranchDayRepo) UpsertOrderPlaced(
	ctx context.Context,
	branchID int64,
	restaurantID int64,
	date string,
	currency string,
	totalMinor int64,
	now time.Time,
) error {
	filter := bson.M{
		"branch_id": branchID,
		"date":      date,
	}
	setFields := bson.M{
		"restaurant_id": restaurantID,
		"updated_at":    now,
	}
	if currency != "" {
		setFields["currency"] = currency
	}
	update := bson.M{
		"$inc": bson.M{
			"orders_count": int64(1),
			"revenue_sum":  totalMinor,
		},
		"$set": setFields,
		"$setOnInsert": bson.M{
			"branch_id":         branchID,
			"date":              date,
			"delivered_count":   int64(0),
			"rejected_count":    int64(0),
			"cancelled_count":   int64(0),
			"delivery_ms_sum":   int64(0),
			"delivery_ms_count": int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// UpsertOrderDelivered increments delivered_count, delivery_ms_sum, and delivery_ms_count for a branch day.
func (r *BranchDayRepo) UpsertOrderDelivered(
	ctx context.Context,
	branchID int64,
	restaurantID int64,
	date string,
	currency string,
	deliveryMs int64,
	now time.Time,
) error {
	filter := bson.M{
		"branch_id": branchID,
		"date":      date,
	}
	setFields := bson.M{
		"restaurant_id": restaurantID,
		"updated_at":    now,
	}
	if currency != "" {
		setFields["currency"] = currency
	}
	update := bson.M{
		"$inc": bson.M{
			"delivered_count":   int64(1),
			"delivery_ms_sum":   deliveryMs,
			"delivery_ms_count": int64(1),
		},
		"$set": setFields,
		"$setOnInsert": bson.M{
			"branch_id":       branchID,
			"date":            date,
			"orders_count":    int64(0),
			"rejected_count":  int64(0),
			"cancelled_count": int64(0),
			"revenue_sum":     int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// UpsertOrderStatusFailure increments rejected_count or cancelled_count for a branch day.
func (r *BranchDayRepo) UpsertOrderStatusFailure(
	ctx context.Context,
	branchID int64,
	restaurantID int64,
	date string,
	currency string,
	isRejected bool,
	now time.Time,
) error {
	filter := bson.M{
		"branch_id": branchID,
		"date":      date,
	}
	incField := "cancelled_count"
	otherField := "rejected_count"
	if isRejected {
		incField = "rejected_count"
		otherField = "cancelled_count"
	}
	setFields := bson.M{
		"restaurant_id": restaurantID,
		"updated_at":    now,
	}
	if currency != "" {
		setFields["currency"] = currency
	}
	update := bson.M{
		"$inc": bson.M{
			incField: int64(1),
		},
		"$set": setFields,
		"$setOnInsert": bson.M{
			"branch_id":         branchID,
			"date":              date,
			"orders_count":      int64(0),
			"delivered_count":   int64(0),
			otherField:          int64(0),
			"revenue_sum":       int64(0),
			"delivery_ms_sum":   int64(0),
			"delivery_ms_count": int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.Update().SetUpsert(true))
	return err
}

// FindByRestaurantAndDateRange returns daily branch rollups for a restaurant (optionally filtered to a single branchID).
// Backed by idx_restaurant_date_branch (restaurant_id, date, branch_id).
func (r *BranchDayRepo) FindByRestaurantAndDateRange(
	ctx context.Context,
	restaurantID int64,
	branchID int64,
	fromDate string,
	toDate string,
) ([]entity.BranchDayDoc, error) {
	filter := bson.M{
		"restaurant_id": restaurantID,
		"date": bson.M{
			"$gte": fromDate,
			"$lte": toDate,
		},
	}
	if branchID > 0 {
		filter["branch_id"] = branchID
	}
	opts := options.Find().SetSort(bson.D{{Key: "date", Value: 1}, {Key: "branch_id", Value: 1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []entity.BranchDayDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// BulkWrite executes a batch of WriteModels in a single round-trip (used by cmd/backfill-aggs).
func (r *BranchDayRepo) BulkWrite(ctx context.Context, models []mongo.WriteModel) error {
	if len(models) == 0 {
		return nil
	}
	_, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}

