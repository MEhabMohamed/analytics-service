// Package analytics — projections.go holds service read projection rows and summaries.
package analytics

// RestaurantDayRow is the service-projected daily restaurant rollup with derived AvgOrderMinor.
type RestaurantDayRow struct {
	Date          string
	OrdersCount   int64
	RevenueMinor  int64
	Currency      string
	AvgOrderMinor int64
}

// RestaurantFailureDayRow holds daily failure counts and derived failureRateBps.
type RestaurantFailureDayRow struct {
	Date           string
	OrdersCount    int64
	RejectedCount  int64
	CancelledCount int64
	FailedCount    int64
	FailureRateBps int64
}

// RestaurantFailuresSummary holds both daily rows and overall range summary.
type RestaurantFailuresSummary struct {
	TotalOrders    int64
	TotalRejected  int64
	TotalCancelled int64
	TotalFailed    int64
	FailureRateBps int64
	Days           []RestaurantFailureDayRow
}

// RestaurantDeliveryAvgDayRow holds daily delivery average metrics derived from sum+count.
type RestaurantDeliveryAvgDayRow struct {
	Date           string
	DeliveredCount int64
	AvgDeliveryMs  int64
	AvgDeliverySec int64
}

// RestaurantDeliveryAvgSummary holds daily and range-overall delivery time metrics.
type RestaurantDeliveryAvgSummary struct {
	TotalDelivered int64
	AvgDeliveryMs  int64
	AvgDeliverySec int64
	Days           []RestaurantDeliveryAvgDayRow
}

// BranchDayRow is the service-projected daily branch rollup.
type BranchDayRow struct {
	BranchID       int64
	RestaurantID   int64
	Date           string
	Currency       string
	OrdersCount    int64
	DeliveredCount int64
	RejectedCount  int64
	CancelledCount int64
	RevenueMinor   int64
	AvgOrderMinor  int64
	AvgDeliveryMs  int64
}

// TopProductRow is the aggregated product performance across a date range.
type TopProductRow struct {
	ProductID    int64  `bson:"_id"`
	QuantitySold int64  `bson:"quantity_sum"`
	RevenueMinor int64  `bson:"revenue_sum"`
	OrdersCount  int64  `bson:"orders_count"`
	Currency     string `bson:"currency"`
}

// PlatformDayRow is the service-projected daily platform rollup.
type PlatformDayRow struct {
	Date               string
	Region             string
	Currency           string
	OrdersCount        int64
	DeliveredCount     int64
	RejectedCount      int64
	CancelledCount     int64
	GMVMinor           int64
	CommissionMinor    int64
	ServiceFeeMinor    int64
	DeliveryFeeMinor   int64
	OnlinePaymentCount int64
	OnlinePaymentMinor int64
	PaymentFailedCount int64
	AvgOrderMinor      int64
	AvgDeliveryMs      int64
}

// ActiveRestaurantsDayRow is the MongoDB aggregation output per day for active restaurants.
type ActiveRestaurantsDayRow struct {
	Date                   string `bson:"_id"`
	ActiveRestaurantsCount int64  `bson:"active_restaurants_count"`
	OrdersCount            int64  `bson:"orders_count"`
	RevenueMinor           int64  `bson:"revenue_sum"`
}

// ActiveRestaurantsSummary holds total distinct active restaurants and per-day breakdown.
type ActiveRestaurantsSummary struct {
	TotalActiveRestaurants int64
	Days                   []ActiveRestaurantsDayRow
}

// PlatformFailureDayRow holds daily platform-wide failure metrics.
type PlatformFailureDayRow struct {
	Date               string
	Region             string
	OrdersCount        int64
	RejectedCount      int64
	CancelledCount     int64
	PaymentFailedCount int64
	OrderFailureBps    int64
}

// PlatformFailuresSummary holds overall and per-day platform failure metrics.
type PlatformFailuresSummary struct {
	TotalOrders        int64
	TotalRejected      int64
	TotalCancelled     int64
	TotalPaymentFailed int64
	OrderFailureBps    int64
	Days               []PlatformFailureDayRow
}

