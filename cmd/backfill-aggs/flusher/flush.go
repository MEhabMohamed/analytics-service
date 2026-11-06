// Package flusher builds MongoDB WriteModels from a batch of Postgres orders and flushes
// all 4 aggregate collections using BulkWrite (zero N+1 queries).
package flusher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"quickbite/analytics-service/app/analytics/repository"
	"quickbite/analytics-service/cmd/backfill-aggs/pgreader"
)

// Repos bundles the 5 MongoDB repositories needed when flushing a backfill batch.
type Repos struct {
	EventIDs *repository.EventIDsRepo
	Rest     *repository.RestaurantDayRepo
	Branch   *repository.BranchDayRepo
	Prod     *repository.ProductDayRepo
	Plat     *repository.PlatformDayRepo
}

// FlushBatchToMongo deduplicates the order batch in bulk (`FilterAndMarkBatchSeen`) and writes
// all rollups across `agg_restaurant_day`, `agg_branch_day`, `agg_product_day`, and `agg_platform_day`
// via 4 unordered `BulkWrite` operations.
func FlushBatchToMongo(
	ctx context.Context,
	defaultRegion string,
	orders []pgreader.OrderRow,
	itemsByOrder map[int64][]pgreader.ItemRow,
	txsByOrder map[int64][]pgreader.TxRow,
	repos Repos,
) error {
	now := time.Now().UTC()

	// Collect all dedupe keys in memory first and batch-dedupe in a single `$in` + `BulkWrite` call
	// (avoids calling MarkSeen inside the `for _, o := range orders` loop, which would cause N+1 queries).
	dedupeKeys := make([]string, 0, len(orders))
	for _, o := range orders {
		if o.Status == "pending_payment" {
			continue
		}
		dedupeKeys = append(dedupeKeys, fmt.Sprintf("backfill:order:%s", o.PublicID))
	}
	freshKeys, err := repos.EventIDs.FilterAndMarkBatchSeen(ctx, dedupeKeys, "backfill.order")
	if err != nil {
		return err
	}

	var restModels []mongo.WriteModel
	var branchModels []mongo.WriteModel
	var prodModels []mongo.WriteModel
	var platModels []mongo.WriteModel

	for _, o := range orders {
		if o.Status == "pending_payment" {
			continue
		}
		dedupeKey := fmt.Sprintf("backfill:order:%s", o.PublicID)
		if !freshKeys[dedupeKey] {
			continue
		}

		date := o.CreatedAt.UTC().Format("2006-01-02")
		reg := strings.ToLower(strings.TrimSpace(o.Region))
		if reg == "" {
			reg = defaultRegion
		}
		curr := strings.ToUpper(strings.TrimSpace(o.Currency))
		if curr == "" {
			curr = "EGP"
		}

		var deliveredInc, rejectedInc, cancelledInc, delivMsInc, delivCountInc int64
		switch o.Status {
		case "delivered":
			deliveredInc = 1
			if o.DeliveredAt.Valid {
				ms := o.DeliveredAt.Time.Sub(o.CreatedAt).Milliseconds()
				if ms > 0 {
					delivMsInc = ms
				}
				delivCountInc = 1
			}
		case "rejected":
			rejectedInc = 1
		case "cancelled":
			cancelledInc = 1
		}

		var codCountInc, codSumInc, onlineCountInc, onlineSumInc, payFailedInc int64
		if strings.EqualFold(o.PaymentMethod, "cod") {
			codCountInc = 1
			codSumInc = o.Total
		}
		for _, tx := range txsByOrder[o.ID] {
			// TS vs Go Syntax Note:
			//   - Go linters (staticcheck QF1003) prefer a "tagged switch" (`switch tx.Status`) over
			//     `if / else if` chains comparing the same variable.
			//   - Unlike JS/TS `switch`, Go `case` blocks do NOT fall through by default—no `break` needed.
			switch tx.Status {
			case "succeeded":
				onlineCountInc++
				onlineSumInc += tx.Amount
			case "failed":
				payFailedInc++
			}
		}

		restModels = append(restModels, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"restaurant_id": o.RestaurantID, "date": date}).
			SetUpdate(bson.M{
				"$inc": bson.M{
					"orders_count":         int64(1),
					"delivered_count":      deliveredInc,
					"rejected_count":       rejectedInc,
					"cancelled_count":      cancelledInc,
					"revenue_sum":          o.Total,
					"online_payment_count": onlineCountInc,
					"online_payment_sum":   onlineSumInc,
					"cod_payment_count":    codCountInc,
					"cod_payment_sum":      codSumInc,
					"delivery_ms_sum":      delivMsInc,
					"delivery_ms_count":    delivCountInc,
				},
				"$set":         bson.M{"currency": curr, "updated_at": now},
				"$setOnInsert": bson.M{"restaurant_id": o.RestaurantID, "date": date},
			}).SetUpsert(true))

		branchModels = append(branchModels, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"branch_id": o.BranchID, "date": date}).
			SetUpdate(bson.M{
				"$inc": bson.M{
					"orders_count":      int64(1),
					"delivered_count":   deliveredInc,
					"rejected_count":    rejectedInc,
					"cancelled_count":   cancelledInc,
					"revenue_sum":       o.Total,
					"delivery_ms_sum":   delivMsInc,
					"delivery_ms_count": delivCountInc,
				},
				"$set":         bson.M{"restaurant_id": o.RestaurantID, "currency": curr, "updated_at": now},
				"$setOnInsert": bson.M{"branch_id": o.BranchID, "date": date},
			}).SetUpsert(true))

		platModels = append(platModels, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"date": date, "region": reg, "currency": curr}).
			SetUpdate(bson.M{
				"$inc": bson.M{
					"orders_count":         int64(1),
					"delivered_count":      deliveredInc,
					"rejected_count":       rejectedInc,
					"cancelled_count":      cancelledInc,
					"gmv_sum":              o.Total,
					"commission_sum":       o.Commission,
					"service_fee_sum":      o.ServiceFee,
					"delivery_fee_sum":     o.DeliveryFee,
					"online_payment_count": onlineCountInc,
					"online_payment_sum":   onlineSumInc,
					"payment_failed_count": payFailedInc,
					"delivery_ms_sum":      delivMsInc,
					"delivery_ms_count":    delivCountInc,
				},
				"$set":         bson.M{"updated_at": now},
				"$setOnInsert": bson.M{"date": date, "region": reg, "currency": curr},
			}).SetUpsert(true))

		for _, it := range itemsByOrder[o.ID] {
			prodModels = append(prodModels, mongo.NewUpdateOneModel().
				SetFilter(bson.M{"branch_id": o.BranchID, "product_id": it.ProductID, "date": date}).
				SetUpdate(bson.M{
					"$inc": bson.M{
						"quantity_sum": it.Quantity,
						"revenue_sum":  it.LineTotal,
						"orders_count": int64(1),
					},
					"$set":         bson.M{"restaurant_id": o.RestaurantID, "currency": curr, "updated_at": now},
					"$setOnInsert": bson.M{"branch_id": o.BranchID, "product_id": it.ProductID, "date": date},
				}).SetUpsert(true))
		}
	}

	if err := repos.Rest.BulkWrite(ctx, restModels); err != nil {
		return err
	}
	if err := repos.Branch.BulkWrite(ctx, branchModels); err != nil {
		return err
	}
	if err := repos.Prod.BulkWrite(ctx, prodModels); err != nil {
		return err
	}
	return repos.Plat.BulkWrite(ctx, platModels)
}

