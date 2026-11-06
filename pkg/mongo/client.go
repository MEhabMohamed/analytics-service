// Package mongo provides a pure connection/disconnection wrapper over the official MongoDB driver.
//
// TS vs Go Syntax Note:
//   - In `order-service` (`lib/knex/knex.ts`), database connections are managed as module-level
//     singletons exported directly.
//   - In Go's `pkg/` layer, global state and environment imports are forbidden. `Connect` takes
//     explicit `(ctx, uri, dbName)` arguments and returns `(*Client, error)` so `lib/boot/boot.go`
//     owns the lifecycle and closes the pool via `defer mongoClient.Disconnect(shutCtx)`.
package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Client wraps the official mongo.Client and target Database handle.
// Strict layering: this package knows how to connect and disconnect ONLY.
// It contains zero collection names or application-specific knowledge.
type Client struct {
	raw *mongo.Client
	db  *mongo.Database
}

// Connect establishes a pooled connection to MongoDB and verifies reachability with Ping.
func Connect(ctx context.Context, uri string, dbName string) (*Client, error) {
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	opts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(50).
		SetMinPoolSize(5).
		SetConnectTimeout(5 * time.Second).
		SetServerSelectionTimeout(5 * time.Second)

	client, err := mongo.Connect(connectCtx, opts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return &Client{
		raw: client,
		db:  client.Database(dbName),
	}, nil
}

// DB returns the underlying *mongo.Database handle for repositories.
func (c *Client) DB() *mongo.Database {
	return c.db
}

// Raw returns the underlying *mongo.Client handle.
func (c *Client) Raw() *mongo.Client {
	return c.raw
}

// Disconnect cleanly closes the MongoDB connection pool.
func (c *Client) Disconnect(ctx context.Context) error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Disconnect(ctx)
}

