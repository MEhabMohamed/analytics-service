// Package pgreader provides keyset-paginated batch SQL readers for cmd/backfill-aggs.
//
// TS vs Go Syntax Note:
//   - In Go, an import statement ALWAYS imports a directory (package) path relative to the module root
//     in `go.mod` (`"quickbite/analytics-service/cmd/backfill-aggs/pgreader"`), never a single `.go` file.
//   - Uppercase identifiers (`OrderRow`, `FetchOrderBatch`) are exported so `cmd/backfill-aggs/main.go`
//     and `cmd/backfill-aggs/flusher` can import and reference them as `pgreader.OrderRow`.
package pgreader

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"
)

// OrderRow represents a historical row from the regional Postgres `orders` table.
type OrderRow struct {
	ID            int64
	PublicID      string
	Region        string
	CountryCode   string
	RestaurantID  int64
	BranchID      int64
	Status        string
	Subtotal      int64
	DeliveryFee   int64
	ServiceFee    int64
	Total         int64
	Commission    int64
	Currency      string
	PaymentMethod string
	CreatedAt     time.Time
	DeliveredAt   sql.NullTime
}

// ItemRow represents a historical line item from `order_items`.
type ItemRow struct {
	OrderID   int64
	ProductID int64
	Quantity  int64
	LineTotal int64
}

// TxRow represents an online charge row from `transactions`.
type TxRow struct {
	OrderID int64
	Amount  int64
	Status  string
}

// FetchOrderBatch streams up to `limit` orders with `id > lastID` in `[fromUTC, toExclusiveUTC)`.
// Uses keyset pagination (`WHERE id > $1 ORDER BY id ASC LIMIT $4`) instead of `OFFSET`.
func FetchOrderBatch(
	ctx context.Context,
	db *sql.DB,
	fromUTC time.Time,
	toExclusiveUTC time.Time,
	lastID int64,
	limit int,
) ([]OrderRow, error) {
	const q = `
		SELECT id, public_id, region, country_code, restaurant_id, branch_id,
		       status, subtotal, delivery_fee, service_fee, total, commission,
		       currency, payment_method, created_at, delivered_at
		FROM orders
		WHERE id > $1 AND created_at >= $2 AND created_at < $3
		ORDER BY id ASC
		LIMIT $4
	`
	rows, err := db.QueryContext(ctx, q, lastID, fromUTC, toExclusiveUTC, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrderRow
	for rows.Next() {
		var r OrderRow
		if err := rows.Scan(
			&r.ID, &r.PublicID, &r.Region, &r.CountryCode, &r.RestaurantID, &r.BranchID,
			&r.Status, &r.Subtotal, &r.DeliveryFee, &r.ServiceFee, &r.Total, &r.Commission,
			&r.Currency, &r.PaymentMethod, &r.CreatedAt, &r.DeliveredAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FetchItemsForOrders fetches all `order_items` for the given `orderIDs` in a single `ANY($1)` query (no N+1).
func FetchItemsForOrders(ctx context.Context, db *sql.DB, orderIDs []int64) (map[int64][]ItemRow, error) {
	out := make(map[int64][]ItemRow, len(orderIDs))
	if len(orderIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT order_id, product_id, quantity, line_total
		FROM order_items
		WHERE order_id = ANY($1)
	`
	rows, err := db.QueryContext(ctx, q, pq.Array(orderIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var it ItemRow
		if err := rows.Scan(&it.OrderID, &it.ProductID, &it.Quantity, &it.LineTotal); err != nil {
			return nil, err
		}
		out[it.OrderID] = append(out[it.OrderID], it)
	}
	return out, rows.Err()
}

// FetchTransactionsForOrders fetches all online charge transactions for the given `orderIDs` in a single `ANY($1)` query (no N+1).
func FetchTransactionsForOrders(ctx context.Context, db *sql.DB, orderIDs []int64) (map[int64][]TxRow, error) {
	out := make(map[int64][]TxRow, len(orderIDs))
	if len(orderIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT order_id, amount, status
		FROM transactions
		WHERE order_id = ANY($1) AND transaction_type = 'charge' AND method = 'online'
	`
	rows, err := db.QueryContext(ctx, q, pq.Array(orderIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tx TxRow
		if err := rows.Scan(&tx.OrderID, &tx.Amount, &tx.Status); err != nil {
			return nil, err
		}
		out[tx.OrderID] = append(out[tx.OrderID], tx)
	}
	return out, rows.Err()
}

