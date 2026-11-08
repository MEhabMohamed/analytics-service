// Package repository is the ONLY package in analytics-service allowed to import `go.mongodb.org/mongo-driver`.
//
// TS vs Go Syntax Note:
//   - In `core-service` and `order-service` (Postgres + Knex), tables and indexes are created via
//     timestamped DDL migration files (`src/migrations/2026...ts`) executed before boot.
//   - MongoDB is schemaless: collections materialize on first write, and `CreateMany` on indexes
//     is idempotent (a no-op if an index with the same specification already exists).
//   - Therefore, DDL migrations are forbidden here; `EnsureIndexes` runs automatically on service boot.
//   - Notice `bson.D` vs `bson.M`: in Go, `bson.M` is an unordered `map[string]any` (whose iteration
//     order is randomized by the Go runtime!), whereas `bson.D` is an ordered slice of key-value pairs.
//     Compound index keys (`(restaurant_id, date)`) and `$sort` stages MUST use `bson.D` because key order matters!
package repository

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"quickbite/analytics-service/app/analytics"
)

const eventIDsTTLSeconds int32 = 7 * 24 * 60 * 60 // 7 days

// EnsureIndexes idempotently creates all required indexes for analytics-service collections on boot.
// Lives in app/analytics/repository/indexes.go (not pkg/mongo) because collection names are app knowledge.
func EnsureIndexes(ctx context.Context, db *mongo.Database) error {
	// 1. agg_restaurant_day
	//    - uq_restaurant_day: supports Upsert + GET /restaurants/:id/days|failures|delivery-avg
	//    - idx_date_restaurant: supports GET /platform/active-restaurants range scan
	if _, err := db.Collection(analytics.CollRestaurantDay).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "restaurant_id", Value: 1}, {Key: "date", Value: 1}},
			Options: options.Index().SetName("uq_restaurant_day").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "date", Value: 1}, {Key: "restaurant_id", Value: 1}},
			Options: options.Index().SetName("idx_date_restaurant"),
		},
	}); err != nil {
		return fmt.Errorf("ensure indexes on %s: %w", analytics.CollRestaurantDay, err)
	}

	// 2. agg_branch_day
	//    - uq_branch_day: supports Upsert per (branch_id, date)
	//    - idx_restaurant_date_branch: supports GET /restaurants/:id/branches/days
	if _, err := db.Collection(analytics.CollBranchDay).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "branch_id", Value: 1}, {Key: "date", Value: 1}},
			Options: options.Index().SetName("uq_branch_day").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "restaurant_id", Value: 1}, {Key: "date", Value: 1}, {Key: "branch_id", Value: 1}},
			Options: options.Index().SetName("idx_restaurant_date_branch"),
		},
	}); err != nil {
		return fmt.Errorf("ensure indexes on %s: %w", analytics.CollBranchDay, err)
	}

	// 3. agg_product_day
	//    - uq_branch_product_day: supports BulkWrite Upsert per (branch_id, product_id, date)
	//    - idx_restaurant_date_revenue: supports GET /restaurants/:id/products/top
	if _, err := db.Collection(analytics.CollProductDay).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "branch_id", Value: 1}, {Key: "product_id", Value: 1}, {Key: "date", Value: 1}},
			Options: options.Index().SetName("uq_branch_product_day").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "restaurant_id", Value: 1}, {Key: "date", Value: 1}, {Key: "revenue_sum", Value: -1}},
			Options: options.Index().SetName("idx_restaurant_date_revenue"),
		},
	}); err != nil {
		return fmt.Errorf("ensure indexes on %s: %w", analytics.CollProductDay, err)
	}

	// 4. agg_platform_day
	//    - uq_platform_day: supports Upsert per (date, region, currency)
	//    - idx_date_region: supports GET /platform/days and GET /platform/failures
	if _, err := db.Collection(analytics.CollPlatformDay).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "date", Value: 1}, {Key: "region", Value: 1}, {Key: "currency", Value: 1}},
			Options: options.Index().SetName("uq_platform_day").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "date", Value: 1}, {Key: "region", Value: 1}},
			Options: options.Index().SetName("idx_date_region"),
		},
	}); err != nil {
		return fmt.Errorf("ensure indexes on %s: %w", analytics.CollPlatformDay, err)
	}

	// 5. event_ids
	//    - uq_event_id: enforces atomic deduplication via InsertOne
	//    - idx_received_at_ttl: automatically purges seen event IDs after 7 days
	if _, err := db.Collection(analytics.CollEventIDs).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "event_id", Value: 1}},
			Options: options.Index().SetName("uq_event_id").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "received_at", Value: 1}},
			Options: options.Index().SetName("idx_received_at_ttl").SetExpireAfterSeconds(eventIDsTTLSeconds),
		},
	}); err != nil {
		return fmt.Errorf("ensure indexes on %s: %w", analytics.CollEventIDs, err)
	}

	return nil
}

