// Package main implements the historical Postgres-to-MongoDB backfill CLI (`cmd/backfill-aggs`).
//
// Divided into 3 cohesive packages:
//   - cmd/backfill-aggs/main.go         : CLI flags, config, Mongo bootstrap, and shard loop
//   - cmd/backfill-aggs/pgreader        : Keyset-paginated batch SQL readers (FetchOrderBatch, FetchItemsForOrders, FetchTransactionsForOrders)
//   - cmd/backfill-aggs/flusher         : Batch deduplication + BulkWrite flush to all 4 MongoDB aggregate collections
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"quickbite/analytics-service/app/analytics/repository"
	"quickbite/analytics-service/cmd/backfill-aggs/flusher"
	"quickbite/analytics-service/cmd/backfill-aggs/pgreader"
	"quickbite/analytics-service/lib/config"
	"quickbite/analytics-service/lib/logger"
	pkgmongo "quickbite/analytics-service/pkg/mongo"
)

func main() {
	log := logger.New()

	fromFlag := flag.String("from", "", "Start date YYYY-MM-DD (inclusive, UTC)")
	toFlag := flag.String("to", "", "End date YYYY-MM-DD (inclusive, UTC)")
	batchSize := flag.Int("batch-size", 500, "Keyset batch size per SQL query")
	pgURLsFlag := flag.String("pg-urls", "", "Comma-separated region=postgres://... pairs (defaults to BACKFILL_PG_URLS env)")
	flag.Parse()

	if *fromFlag == "" || *toFlag == "" {
		log.Error("missing required flags: -from=YYYY-MM-DD -to=YYYY-MM-DD")
		os.Exit(1)
	}

	fromTime, err := time.Parse("2006-01-02", *fromFlag)
	if err != nil {
		log.Error("invalid -from date", "error", err.Error())
		os.Exit(1)
	}
	toTime, err := time.Parse("2006-01-02", *toFlag)
	if err != nil || toTime.Before(fromTime) {
		log.Error("invalid -to date (must be >= -from)")
		os.Exit(1)
	}
	toExclusive := toTime.UTC().AddDate(0, 0, 1)

	cfg, err := config.Load()
	if err != nil {
		log.Error("load config failed", "error", err.Error())
		os.Exit(1)
	}

	rawURLs := *pgURLsFlag
	if rawURLs == "" {
		rawURLs = cfg.BackfillPGURLs
	}
	shards, err := parseShardURLs(rawURLs)
	if err != nil {
		log.Error("invalid shard DSNs", "error", err.Error())
		os.Exit(1)
	}

	ctx := context.Background()
	mongoClient, err := pkgmongo.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Error("mongo connect failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = mongoClient.Disconnect(ctx) }()

	if err := repository.EnsureIndexes(ctx, mongoClient.DB()); err != nil {
		log.Error("ensure indexes failed", "error", err.Error())
		os.Exit(1)
	}

	repos := flusher.Repos{
		EventIDs: repository.NewEventIDsRepo(mongoClient.DB()),
		Rest:     repository.NewRestaurantDayRepo(mongoClient.DB()),
		Branch:   repository.NewBranchDayRepo(mongoClient.DB()),
		Prod:     repository.NewProductDayRepo(mongoClient.DB()),
		Plat:     repository.NewPlatformDayRepo(mongoClient.DB()),
	}

	var totalOrdersProcessed int64
	for region, dsn := range shards {
		n, err := backfillShard(ctx, log, region, dsn, fromTime.UTC(), toExclusive, *batchSize, repos)
		if err != nil {
			log.Error("shard backfill failed", "region", region, "error", err.Error())
			os.Exit(1)
		}
		totalOrdersProcessed += n
	}

	log.Info("backfill completed", "totalOrdersProcessed", totalOrdersProcessed)
}

func backfillShard(
	ctx context.Context,
	log *slog.Logger,
	region string,
	dsn string,
	fromUTC time.Time,
	toExclusiveUTC time.Time,
	batchSize int,
	repos flusher.Repos,
) (int64, error) {
	pgDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return 0, err
	}
	defer pgDB.Close()

	var lastID int64
	var processed int64

	for {
		orders, err := pgreader.FetchOrderBatch(ctx, pgDB, fromUTC, toExclusiveUTC, lastID, batchSize)
		if err != nil {
			return processed, err
		}
		if len(orders) == 0 {
			break
		}

		orderIDs := make([]int64, 0, len(orders))
		for _, o := range orders {
			orderIDs = append(orderIDs, o.ID)
			if o.ID > lastID {
				lastID = o.ID
			}
		}

		itemsByOrder, err := pgreader.FetchItemsForOrders(ctx, pgDB, orderIDs)
		if err != nil {
			return processed, err
		}

		txsByOrder, err := pgreader.FetchTransactionsForOrders(ctx, pgDB, orderIDs)
		if err != nil {
			return processed, err
		}

		if err := flusher.FlushBatchToMongo(ctx, region, orders, itemsByOrder, txsByOrder, repos); err != nil {
			return processed, err
		}

		processed += int64(len(orders))
		log.Info("backfill batch flushed", "region", region, "batchCount", len(orders), "totalProcessed", processed)
	}

	return processed, nil
}

func parseShardURLs(raw string) (map[string]string, error) {
	out := make(map[string]string)
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, '=')
		if idx <= 0 {
			return nil, fmt.Errorf("expected region=dsn format, got %q", part)
		}
		out[strings.ToLower(strings.TrimSpace(part[:idx]))] = strings.TrimSpace(part[idx+1:])
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no shard DSNs provided; pass -pg-urls or set BACKFILL_PG_URLS")
	}
	return out, nil
}
