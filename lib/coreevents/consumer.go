// Package coreevents implements the generic inbound event consumer with idempotency and DLQ routing.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/lib/core-events/consumer.ts` (`startCoreEventsConsumer`, `handleMessage`).
//   - Notice `json.RawMessage` on `Envelope.Payload`: in Go, `json.RawMessage` defers parsing the inner
//     `payload` object until the specific `EventHandler` knows which concrete struct to unmarshal into.
package coreevents

import (
	"context"
	"encoding/json"
	"log/slog"

	"quickbite/analytics-service/pkg/messaging"
)

// Consumer routes inbound AMQP deliveries through idempotency deduplication and registered EventHandlers.
// Mirrors order-service/src/lib/core-events/consumer.ts.
type Consumer struct {
	broker   messaging.Broker
	deduper  EventDeduper
	log      *slog.Logger
	handlers map[string]EventHandler
}

// NewConsumer constructs a Consumer with the given Broker, EventDeduper, and Logger.
func NewConsumer(broker messaging.Broker, deduper EventDeduper, log *slog.Logger) *Consumer {
	return &Consumer{
		broker:   broker,
		deduper:  deduper,
		log:      log,
		handlers: make(map[string]EventHandler),
	}
}

// Register binds an eventType (e.g. "order.placed") to its EventHandler.
func (c *Consumer) Register(eventType string, handler EventHandler) {
	c.handlers[eventType] = handler
}

// Start declares the topology and starts consuming deliveries on the given queue.
func (c *Consumer) Start(ctx context.Context, top messaging.Topology) error {
	if err := c.broker.DeclareTopology(top); err != nil {
		return err
	}
	if err := c.broker.Consume(ctx, top, c.handleDelivery); err != nil {
		return err
	}
	return nil
}

func (c *Consumer) handleDelivery(ctx context.Context, d messaging.Delivery) {
	var env Envelope
	if err := json.Unmarshal(d.Body, &env); err != nil || env.EventID == "" || env.EventType == "" {
		c.log.Error("malformed event envelope, sending to DLQ",
			"routingKey", d.RoutingKey,
		)
		_ = d.Nack(false)
		return
	}

	// Unwrap stringified JSON payload if double-encoded by Postgres JSONB string insertion
	env.Payload = unwrapPayload(env.Payload)

	handler, exists := c.handlers[env.EventType]
	if !exists {
		c.log.Warn("unknown event type, acking and skipping",
			"eventType", env.EventType,
			"eventId", env.EventID,
		)
		_ = d.Ack()
		return
	}

	if c.deduper != nil {
		fresh, err := c.deduper.MarkSeen(ctx, env.EventID, env.EventType)
		if err != nil {
			c.log.Error("deduper failed, sending to DLQ",
				"eventType", env.EventType,
				"eventId", env.EventID,
				"error", err.Error(),
			)
			_ = d.Nack(false)
			return
		}
		if !fresh {
			c.log.Info("duplicate event skipped",
				"eventType", env.EventType,
				"eventId", env.EventID,
			)
			_ = d.Ack()
			return
		}
	}

	if err := handler(ctx, env); err != nil {
		c.log.Error("event handler failed, sending to DLQ",
			"eventType", env.EventType,
			"eventId", env.EventID,
			"error", err.Error(),
		)
		_ = d.Nack(false)
		return
	}

	_ = d.Ack()
}

func unwrapPayload(raw json.RawMessage) json.RawMessage {
	if len(raw) > 1 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return json.RawMessage(s)
		}
	}
	return raw
}

