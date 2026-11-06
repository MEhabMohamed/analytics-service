// Package entity defines plain MongoDB document structs with `bson` tags.
//
// TS vs Go Syntax Note:
//   - Replaces the Redis `SETNX` key (`core-events:dedupe:<eventId>`) used in `order-service`
//     with a durable MongoDB document backed by a unique index on `event_id` and a 7-day TTL
//     index on `received_at`. Why? Because `order-service` handlers are idempotent cache
//     deletions, whereas `analytics-service` handlers perform non-idempotent `$inc` math on
//     monetary counters—if a dedupe store evaporated on restart, replayed events would double-count revenue!
package entity

import "time"

// EventIDDoc models a document in the event_ids idempotency collection.
type EventIDDoc struct {
	EventID    string    `bson:"event_id"`
	EventType  string    `bson:"event_type"`
	ReceivedAt time.Time `bson:"received_at"`
}

