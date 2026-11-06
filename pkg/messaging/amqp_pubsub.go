// Package messaging — amqp_pubsub.go implements Consume and Publish on AMQPBroker.
//
// TS vs Go Syntax Note:
//   - In Node.js (`amqplib`), `channel.consume(queue, (msg) => ...)` registers an event-loop callback.
//   - In Go (`amqp091-go`), `ch.Consume(...)` returns a read-only Go channel (`<-chan amqp.Delivery`).
//     We spawn a lightweight goroutine (`go func() { for { select { ... } } }()`) that reads from the
//     delivery channel until `ctx.Done()` fires on shutdown.
package messaging

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Consume starts a background goroutine reading deliveries from top.Queue until ctx is cancelled.
func (b *AMQPBroker) Consume(ctx context.Context, top Topology, handler HandlerFunc) error {
	b.mu.Lock()
	if b.conn == nil || b.conn.IsClosed() {
		b.mu.Unlock()
		return fmt.Errorf("amqp not connected")
	}

	ch, err := b.conn.Channel()
	if err != nil {
		b.mu.Unlock()
		return fmt.Errorf("open consumer channel: %w", err)
	}
	b.consChs = append(b.consChs, ch)
	b.mu.Unlock()

	prefetch := top.Prefetch
	if prefetch <= 0 {
		prefetch = 32
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := ch.Consume(
		top.Queue,
		"",    // consumer tag (auto)
		false, // autoAck = false (manual ack required)
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume %s: %w", top.Queue, err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-deliveries:
				if !ok {
					return
				}
				d := Delivery{
					RoutingKey: msg.RoutingKey,
					Body:       msg.Body,
					Ack: func() error {
						return msg.Ack(false)
					},
					Nack: func(requeue bool) error {
						return msg.Nack(false, requeue)
					},
				}
				handler(ctx, d)
			}
		}
	}()

	return nil
}

// Publish sends a persistent JSON message to the given exchange and routing key.
func (b *AMQPBroker) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.pubCh == nil {
		return fmt.Errorf("amqp publisher channel not initialized")
	}

	return b.pubCh.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         body,
		},
	)
}

