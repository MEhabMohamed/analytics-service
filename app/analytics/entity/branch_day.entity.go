// Package entity defines plain MongoDB document structs with `bson` tags.
//
// TS vs Go Syntax Note:
//   - Unlike TypeScript classes that use `Date` objects (`createdAt!: Date`), Go uses `time.Time`
//     from the standard `time` package. The `go.mongodb.org/mongo-driver` automatically serializes
//     `time.Time` to/from BSON UTC DateTime values.
package entity

import "time"

// BranchDayDoc models a document in the agg_branch_day collection.
type BranchDayDoc struct {
	BranchID        int64     `bson:"branch_id"`
	RestaurantID    int64     `bson:"restaurant_id"`
	Date            string    `bson:"date"` // "YYYY-MM-DD" UTC
	Currency        string    `bson:"currency"`
	OrdersCount     int64     `bson:"orders_count"`
	DeliveredCount  int64     `bson:"delivered_count"`
	RejectedCount   int64     `bson:"rejected_count"`
	CancelledCount  int64     `bson:"cancelled_count"`
	RevenueSum      int64     `bson:"revenue_sum"`
	DeliveryMsSum   int64     `bson:"delivery_ms_sum"`
	DeliveryMsCount int64     `bson:"delivery_ms_count"`
	UpdatedAt       time.Time `bson:"updated_at"`
}

