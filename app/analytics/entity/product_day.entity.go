// Package entity defines plain MongoDB document structs with `bson` tags.
//
// TS vs Go Syntax Note:
//   - Each field uses `int64` + `bson:"snake_case"` tags so `mongo-driver` decodes documents
//     directly into `ProductDayDoc` without a manual `toEntity(row)` helper.
package entity

import "time"

// ProductDayDoc models a document in the agg_product_day collection.
type ProductDayDoc struct {
	ProductID    int64     `bson:"product_id"`
	RestaurantID int64     `bson:"restaurant_id"`
	BranchID     int64     `bson:"branch_id"`
	Date         string    `bson:"date"` // "YYYY-MM-DD" UTC
	Currency     string    `bson:"currency"`
	QuantitySum  int64     `bson:"quantity_sum"`
	RevenueSum   int64     `bson:"revenue_sum"`
	OrdersCount  int64     `bson:"orders_count"`
	UpdatedAt    time.Time `bson:"updated_at"`
}

