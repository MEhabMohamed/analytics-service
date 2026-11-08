// Package repository implements MongoDB data access for agg_product_day.
//
// TS vs Go Syntax Note (Avoiding N+1 with BulkWrite):
//   - In `order-service` (`order-item.repo.ts`), `bulkInsertItems` sends one multi-row SQL
//     `INSERT INTO order_items VALUES (...), (...)` so N line items take 1 DB round-trip.
//   - In MongoDB, the equivalent for N upserts is `coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))`
//     where each element of `[]mongo.WriteModel` is a `mongo.NewUpdateOneModel().SetUpsert(true)`.
//   - Never call `UpdateOne` inside a `for _, item := range items` loop — that would be an N+1 query!
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"quickbite/analytics-service/app/analytics"
)

// ProductDayRepo manages the agg_product_day collection.
type ProductDayRepo struct {
	coll *mongo.Collection
}

// NewProductDayRepo constructs a ProductDayRepo.
func NewProductDayRepo(db *mongo.Database) *ProductDayRepo {
	return &ProductDayRepo{
		coll: db.Collection(analytics.CollProductDay),
	}
}

// BulkUpsertOrderItems updates agg_product_day for all line items of an order in a SINGLE BulkWrite round-trip (zero N+1).
// If the same productId appears multiple times in the order's items slice, quantities and lineTotals are coalesced first
// so orders_count increments by 1 per order per distinct product.
func (r *ProductDayRepo) BulkUpsertOrderItems(
	ctx context.Context,
	restaurantID int64,
	branchID int64,
	date string,
	currency string,
	items []analytics.OrderPlacedItemInput,
	now time.Time,
) error {
	if len(items) == 0 {
		return nil
	}

	// Coalesce duplicate productIds within the same order
	qtyByProduct := make(map[int64]int64, len(items))
	revByProduct := make(map[int64]int64, len(items))
	orderProductIDs := make([]int64, 0, len(items))

	for _, it := range items {
		if it.ProductID <= 0 {
			continue
		}
		if _, exists := qtyByProduct[it.ProductID]; !exists {
			orderProductIDs = append(orderProductIDs, it.ProductID)
		}
		qtyByProduct[it.ProductID] += it.Quantity
		revByProduct[it.ProductID] += it.LineTotal
	}

	if len(orderProductIDs) == 0 {
		return nil
	}

	models := make([]mongo.WriteModel, 0, len(orderProductIDs))
	for _, pid := range orderProductIDs {
		filter := bson.M{
			"branch_id":  branchID,
			"product_id": pid,
			"date":       date,
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
				"quantity_sum": qtyByProduct[pid],
				"revenue_sum":  revByProduct[pid],
				"orders_count": int64(1),
			},
			"$set": setFields,
			"$setOnInsert": bson.M{
				"branch_id":  branchID,
				"product_id": pid,
				"date":       date,
			},
		}
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(update).
			SetUpsert(true))
	}

	_, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}

// FindTopProducts aggregates top products by revenue_sum across [fromDate, toDate] for a restaurant (and optional branch)
// in a single MongoDB aggregation pipeline (zero N+1).
// Backed by idx_restaurant_date_revenue (restaurant_id, date, revenue_sum DESC).
func (r *ProductDayRepo) FindTopProducts(
	ctx context.Context,
	restaurantID int64,
	branchID int64,
	fromDate string,
	toDate string,
	limit int64,
) ([]analytics.TopProductRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}

	match := bson.M{
		"restaurant_id": restaurantID,
		"date": bson.M{
			"$gte": fromDate,
			"$lte": toDate,
		},
	}
	if branchID > 0 {
		match["branch_id"] = branchID
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id":          "$product_id",
			"quantity_sum": bson.M{"$sum": "$quantity_sum"},
			"revenue_sum":  bson.M{"$sum": "$revenue_sum"},
			"orders_count": bson.M{"$sum": "$orders_count"},
			"currency":     bson.M{"$first": "$currency"},
		}}},
		{{Key: "$sort", Value: bson.D{
			{Key: "revenue_sum", Value: -1},
			{Key: "quantity_sum", Value: -1},
			{Key: "_id", Value: 1},
		}}},
		{{Key: "$limit", Value: limit}},
	}

	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []analytics.TopProductRow
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// BulkWrite executes a batch of WriteModels in a single round-trip (used by cmd/backfill-aggs).
func (r *ProductDayRepo) BulkWrite(ctx context.Context, models []mongo.WriteModel) error {
	if len(models) == 0 {
		return nil
	}
	_, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}
