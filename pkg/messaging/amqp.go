// Package messaging implements the Broker interface on top of github.com/rabbitmq/amqp091-go.
//
// Divided into 3 cohesive files in `package messaging`:
//   - types.go       : Broker interface, Topology, Delivery, HandlerFunc
//   - amqp.go        : AMQPBroker struct, Connect, DeclareTopology, Close
//   - amqp_pubsub.go : Consume (goroutine channel reader) and Publish
//
// TS vs Go Syntax Note:
//   - A `sync.Mutex` (`b.mu`) guards connection/channel mutations across concurrent goroutines.
package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// AMQPBroker implements Broker on top of github.com/rabbitmq/amqp091-go.
// It contains zero application or domain knowledge.
type AMQPBroker struct {
	url     string
	mu      sync.Mutex
	conn    *amqp.Connection
	pubCh   *amqp.Channel
	consChs []*amqp.Channel
}

// NewAMQPBroker constructs a new AMQPBroker for the given AMQP URI.
func NewAMQPBroker(url string) *AMQPBroker {
	return &AMQPBroker{
		url: url,
	}
}

// Connect opens the underlying AMQP connection and a publisher channel.
func (b *AMQPBroker) Connect(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn != nil && !b.conn.IsClosed() {
		return nil
	}

	conn, err := amqp.DialConfig(b.url, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
	})
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("amqp open publish channel: %w", err)
	}

	b.conn = conn
	b.pubCh = pubCh
	return nil
}

// DeclareTopology idempotently declares the topic exchange, optional DLX/DLQ, main queue, and bindings.
func (b *AMQPBroker) DeclareTopology(top Topology) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.conn == nil || b.conn.IsClosed() {
		return fmt.Errorf("amqp not connected")
	}

	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("amqp channel: %w", err)
	}
	defer ch.Close()

	// 1. Primary topic exchange
	if err := ch.ExchangeDeclare(top.Exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange %s: %w", top.Exchange, err)
	}

	// 2. Dead-letter exchange & queue if configured.
	// Uses "topic" (with "#" wildcard binding) to match core-service / order-service DLX declarations,
	// falling back to "fanout" on a fresh channel if the DLX was previously created as fanout.
	queueArgs := amqp.Table{}
	if top.DeadLetterExchange != "" {
		if err := ch.ExchangeDeclare(top.DeadLetterExchange, "topic", true, false, false, false, nil); err != nil {
			// A 406 PRECONDITION_FAILED closes `ch`; re-open a channel and retry as "fanout".
			ch, err = b.conn.Channel()
			if err != nil {
				return fmt.Errorf("amqp reopen channel for dlx %s: %w", top.DeadLetterExchange, err)
			}
			defer ch.Close()
			if err := ch.ExchangeDeclare(top.DeadLetterExchange, "fanout", true, false, false, false, nil); err != nil {
				return fmt.Errorf("declare dlx %s: %w", top.DeadLetterExchange, err)
			}
		}
		queueArgs["x-dead-letter-exchange"] = top.DeadLetterExchange

		if top.DeadLetterQueue != "" {
			if _, err := ch.QueueDeclare(top.DeadLetterQueue, true, false, false, false, nil); err != nil {
				return fmt.Errorf("declare dlq %s: %w", top.DeadLetterQueue, err)
			}
			if err := ch.QueueBind(top.DeadLetterQueue, "#", top.DeadLetterExchange, false, nil); err != nil {
				return fmt.Errorf("bind dlq %s: %w", top.DeadLetterQueue, err)
			}
		}
	}

	// 3. Main consumer queue
	if top.Queue != "" {
		if _, err := ch.QueueDeclare(top.Queue, true, false, false, false, queueArgs); err != nil {
			return fmt.Errorf("declare queue %s: %w", top.Queue, err)
		}
		for _, key := range top.BindingKeys {
			if key == "" {
				continue
			}
			if err := ch.QueueBind(top.Queue, key, top.Exchange, false, nil); err != nil {
				return fmt.Errorf("bind queue %s to %s (%s): %w", top.Queue, top.Exchange, key, err)
			}
		}
	}

	return nil
}

// Close closes all open consumer/publisher channels and the underlying connection.
func (b *AMQPBroker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, ch := range b.consChs {
		_ = ch.Close()
	}
	b.consChs = nil

	if b.pubCh != nil {
		_ = b.pubCh.Close()
		b.pubCh = nil
	}
	if b.conn != nil && !b.conn.IsClosed() {
		err := b.conn.Close()
		b.conn = nil
		return err
	}
	return nil
}
