// Package mongo provides a pure connection/disconnection wrapper over the official MongoDB v2 driver.
//
// TS vs Go Syntax Note:
//   - In `order-service` (`lib/knex/knex.ts`), database connections are managed as module-level
//     singletons exported directly.
//   - In Go's `pkg/` layer, global state and environment imports are forbidden. `Connect` takes
//     explicit `(ctx, uri, dbName)` arguments and returns `(*Client, error)` so `lib/boot/boot.go`
//     owns the lifecycle and closes the pool via `defer mongoClient.Disconnect(shutCtx)`.
//   - Uses `go.mongodb.org/mongo-driver/v2/mongo` (`mongo.Connect(opts)` + `client.Ping(ctx, readpref.Primary())`).
package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Client wraps the official v2 mongo.Client and target Database handle.
// Strict layering: this package knows how to connect and disconnect ONLY.
// It contains zero collection names or application-specific knowledge.
type Client struct {
	raw *mongo.Client
	db  *mongo.Database
}

// Connect establishes a pooled connection to MongoDB and verifies reachability with Ping.
func Connect(ctx context.Context, uri string, dbName string) (*Client, error) {
	opts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(50).
		SetMinPoolSize(5).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)

	// Create a new client and connect to the server (mongo-driver v2 API)
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	// Send a ping to confirm a successful connection
	pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
	defer pingCancel()
	if err := client.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo ping: %w", err)
	}
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")

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
