// Package coreevents provides the generic RabbitMQ event consumer and deduplication contract.
//
// TS vs Go Syntax Note (Dependency Inversion via Duck-Typed Interface):
//   - `lib/coreevents` is in the `lib/` layer, which is forbidden from importing `app/analytics/repository`.
//   - By declaring the 1-method `EventDeduper` interface here in `lib/coreevents`, `repository.EventIDsRepo`
//     in `app/analytics/repository` satisfies it implicitly without either package importing the other!
package coreevents

import (
	"context"
	"encoding/json"
)

// Envelope is the generic wire format published by core-service and order-service outbox drainers.
type Envelope struct {
	EventID       string          `json:"eventId"`
	EventType     string          `json:"eventType"`
	OccurredAt    string          `json:"occurredAt"`
	AggregateType string          `json:"aggregateType"`
	AggregateID   string          `json:"aggregateId"`
	Region        string          `json:"region,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// EventHandler processes the raw JSON payload of a specific event type.
type EventHandler func(ctx context.Context, env Envelope) error

// EventDeduper is the dependency-inverted idempotency interface satisfied by app/analytics/repository.EventIDsRepo.
// MarkSeen returns fresh=true if the eventId was recorded for the first time, or fresh=false if it is a duplicate.
type EventDeduper interface {
	MarkSeen(ctx context.Context, eventID string, eventType string) (bool, error)
}

