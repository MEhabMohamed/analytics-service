// Package entity defines plain MongoDB document structs with `bson` tags.
//
// TS vs Go Syntax Note:
//   - In TypeScript (`order-service/src/app/order/entity/order.entity.ts`), entities are classes
//     taking `Partial<OrderEntity>` in their constructor, and repositories manually map `snake_case`
//     SQL columns to `camelCase` properties in a private `toEntity(row)` function.
//   - In Go with `mongo-driver`, `bson:"snake_case"` struct tags let the driver decode MongoDB BSON
//     documents directly into typed Go structs without writing a manual `toEntity` mapper!
//   - We use `int64` for all counters and minor-unit money amounts (replacing JS `number` which is a
//     64-bit float limited to 2^53 - 1).
package entity

import "time"

// RestaurantDayDoc models a document in the agg_restaurant_day collection.
// Averages are stored as associative sum+count pairs so replays and out-of-order events merge cleanly.
type RestaurantDayDoc struct {
	RestaurantID       int64     `bson:"restaurant_id"`
	Date               string    `bson:"date"` // "YYYY-MM-DD" UTC
	Currency           string    `bson:"currency"`
	OrdersCount        int64     `bson:"orders_count"`
	DeliveredCount     int64     `bson:"delivered_count"`
	RejectedCount      int64     `bson:"rejected_count"`
	CancelledCount     int64     `bson:"cancelled_count"`
	RevenueSum         int64     `bson:"revenue_sum"`
	OnlinePaymentCount int64     `bson:"online_payment_count"`
	OnlinePaymentSum   int64     `bson:"online_payment_sum"`
	CODPaymentCount    int64     `bson:"cod_payment_count"`
	CODPaymentSum      int64     `bson:"cod_payment_sum"`
	DeliveryMsSum      int64     `bson:"delivery_ms_sum"`
	DeliveryMsCount    int64     `bson:"delivery_ms_count"`
	UpdatedAt          time.Time `bson:"updated_at"`
}

