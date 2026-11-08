// Package repository — restaurant_day_queries.repo.go contains delivery/failure upserts and read queries for RestaurantDayRepo.
package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"quickbite/analytics-service/app/analytics"
	"quickbite/analytics-service/app/analytics/entity"
)

// UpsertOrderDelivered atomically increments delivered_count, delivery_ms_sum, and delivery_ms_count.
// Out-of-order safe: if delivered arrives before placed, $setOnInsert initializes orders_count/revenue_sum to 0
// so the subsequent order.placed event increments them without data loss.
func (r *RestaurantDayRepo) UpsertOrderDelivered(
	ctx context.Context,
	restaurantID int64,
	date string,
	currency string,
	deliveryMs int64,
	now time.Time,
) error {
	filter := bson.M{
		"restaurant_id": restaurantID,
		"date":          date,
	}
	setFields := bson.M{"updated_at": now}
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
			"restaurant_id":        restaurantID,
			"date":                 date,
			"orders_count":         int64(0),
			"rejected_count":       int64(0),
			"cancelled_count":      int64(0),
			"revenue_sum":          int64(0),
			"online_payment_count": int64(0),
			"online_payment_sum":   int64(0),
			"cod_payment_count":    int64(0),
			"cod_payment_sum":      int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}

// UpsertOrderStatusFailure atomically increments rejected_count or cancelled_count.
func (r *RestaurantDayRepo) UpsertOrderStatusFailure(
	ctx context.Context,
	restaurantID int64,
	date string,
	currency string,
	isRejected bool,
	now time.Time,
) error {
	filter := bson.M{
		"restaurant_id": restaurantID,
		"date":          date,
	}
	incField := "cancelled_count"
	otherField := "rejected_count"
	if isRejected {
		incField = "rejected_count"
		otherField = "cancelled_count"
	}
	setFields := bson.M{"updated_at": now}
	if currency != "" {
		setFields["currency"] = currency
	}
	update := bson.M{
		"$inc": bson.M{
			incField: int64(1),
		},
		"$set": setFields,
		"$setOnInsert": bson.M{
			"restaurant_id":        restaurantID,
			"date":                 date,
			"orders_count":         int64(0),
			"delivered_count":      int64(0),
			otherField:             int64(0),
			"revenue_sum":          int64(0),
			"online_payment_count": int64(0),
			"online_payment_sum":   int64(0),
			"cod_payment_count":    int64(0),
			"cod_payment_sum":      int64(0),
			"delivery_ms_sum":      int64(0),
			"delivery_ms_count":    int64(0),
		},
	}
	_, err := r.coll.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}

// FindByRestaurantAndDateRange returns daily documents for a restaurant in [fromDate, toDate] sorted by date ASC.
// Backed by uq_restaurant_day (restaurant_id, date).
func (r *RestaurantDayRepo) FindByRestaurantAndDateRange(
	ctx context.Context,
	restaurantID int64,
	fromDate string,
	toDate string,
) ([]entity.RestaurantDayDoc, error) {
	filter := bson.M{
		"restaurant_id": restaurantID,
		"date": bson.M{
			"$gte": fromDate,
			"$lte": toDate,
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "date", Value: 1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var docs []entity.RestaurantDayDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// AggregateActiveRestaurants computes per-day active restaurant counts and total distinct active restaurants
// across [fromDate, toDate] using a single MongoDB aggregation pipeline (no N+1).
func (r *RestaurantDayRepo) AggregateActiveRestaurants(
	ctx context.Context,
	fromDate string,
	toDate string,
) (int64, []analytics.ActiveRestaurantsDayRow, error) {
	matchStage := bson.D{{Key: "$match", Value: bson.M{
		"date": bson.M{
			"$gte": fromDate,
			"$lte": toDate,
		},
		"orders_count": bson.M{"$gt": 0},
	}}}

	pipeline := mongo.Pipeline{
		matchStage,
		{{Key: "$group", Value: bson.M{
			"_id":                      "$date",
			"active_restaurants_count": bson.M{"$sum": 1},
			"orders_count":             bson.M{"$sum": "$orders_count"},
			"revenue_sum":              bson.M{"$sum": "$revenue_sum"},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, nil, err
	}
	defer cur.Close(ctx)

	var days []analytics.ActiveRestaurantsDayRow
	if err := cur.All(ctx, &days); err != nil {
		return 0, nil, err
	}

	// Distinct restaurant_ids across the full range (mongo-driver v2 returns *mongo.DistinctResult)
	var distinctVals []int64
	if err := r.coll.Distinct(ctx, "restaurant_id", bson.M{
		"date": bson.M{
			"$gte": fromDate,
			"$lte": toDate,
		},
		"orders_count": bson.M{"$gt": 0},
	}).Decode(&distinctVals); err != nil {
		return 0, nil, err
	}

	return int64(len(distinctVals)), days, nil
}

// BulkWrite executes a batch of WriteModels in a single round-trip (used by cmd/backfill-aggs).
func (r *RestaurantDayRepo) BulkWrite(ctx context.Context, models []mongo.WriteModel) error {
	if len(models) == 0 {
		return nil
	}
	_, err := r.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}
