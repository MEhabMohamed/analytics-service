package helpers

import (
	"context"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"quickbite/analytics-service/app/analytics/repository"
	"quickbite/analytics-service/app/analytics/service"
	"quickbite/analytics-service/lib/config"
	pkgmongo "quickbite/analytics-service/pkg/mongo"
)

// SetupTestDB connects to the test MongoDB database from .env.test, ensures all indexes,
// truncates all collections for a clean state, and registers cleanup on t.Cleanup.
func SetupTestDB(t *testing.T, cfg *config.Config) (*mongo.Database, *service.Service, *repository.EventIDsRepo) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := pkgmongo.Connect(ctx, cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		t.Fatalf("connect test mongo (%s / %s): %v", cfg.MongoURI, cfg.MongoDB, err)
	}

	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		_ = client.Disconnect(shutCtx)
	})

	db := client.DB()
	if err := repository.EnsureIndexes(ctx, db); err != nil {
		t.Fatalf("ensure indexes: %v", err)
	}

	TruncateAll(t, db)

	eventIDsRepo := repository.NewEventIDsRepo(db)
	restaurantRepo := repository.NewRestaurantDayRepo(db)
	branchRepo := repository.NewBranchDayRepo(db)
	productRepo := repository.NewProductDayRepo(db)
	platformRepo := repository.NewPlatformDayRepo(db)

	svc := service.New(restaurantRepo, branchRepo, productRepo, platformRepo)
	return db, svc, eventIDsRepo
}

// TruncateAll clears all documents from every analytics collection in the test DB.
func TruncateAll(t *testing.T, db *mongo.Database) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collections := []string{
		"agg_restaurant_day",
		"agg_branch_day",
		"agg_product_day",
		"agg_platform_day",
		"event_ids",
	}
	for _, col := range collections {
		if _, err := db.Collection(col).DeleteMany(ctx, bson.D{}); err != nil {
			t.Fatalf("truncate %s: %v", col, err)
		}
	}
}
