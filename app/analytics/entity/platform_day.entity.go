// Package entity defines plain MongoDB document structs with `bson` tags.
//
// TS vs Go Syntax Note:
//   - Models platform-wide daily metrics per `(date, region, currency)` in `agg_platform_day`.
//   - No ORM/ODM annotations (`@Entity`, `@Column`) are used—just plain Go structs and `bson` tags.
package entity

import "time"

// PlatformDayDoc models a document in the agg_platform_day collection.
type PlatformDayDoc struct {
	Date               string    `bson:"date"` // "YYYY-MM-DD" UTC
	Region             string    `bson:"region"`
	Currency           string    `bson:"currency"`
	OrdersCount        int64     `bson:"orders_count"`
	DeliveredCount     int64     `bson:"delivered_count"`
	RejectedCount      int64     `bson:"rejected_count"`
	CancelledCount     int64     `bson:"cancelled_count"`
	GMVSum             int64     `bson:"gmv_sum"`
	CommissionSum      int64     `bson:"commission_sum"`
	ServiceFeeSum      int64     `bson:"service_fee_sum"`
	DeliveryFeeSum     int64     `bson:"delivery_fee_sum"`
	OnlinePaymentCount int64     `bson:"online_payment_count"`
	OnlinePaymentSum   int64     `bson:"online_payment_sum"`
	PaymentFailedCount int64     `bson:"payment_failed_count"`
	DeliveryMsSum      int64     `bson:"delivery_ms_sum"`
	DeliveryMsCount    int64     `bson:"delivery_ms_count"`
	UpdatedAt          time.Time `bson:"updated_at"`
}

