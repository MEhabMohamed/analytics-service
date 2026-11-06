// Package messaging defines the framework-free message broker contract and AMQP 0-9-1 implementation.
//
// TS vs Go Syntax Note:
//   - Mirrors `order-service/src/pkg/messaging/message-broker.interface.ts` (`IMessageBroker`).
//   - In Go, interfaces do not use an `I` prefix (`Broker` instead of `IMessageBroker`), and
//     `[]byte` replaces Node's `Buffer`.
package messaging

import "context"

// Delivery represents an inbound AMQP message with manual acknowledgment controls.
type Delivery struct {
	RoutingKey string
	Body       []byte
	Ack        func() error
	Nack       func(requeue bool) error
}

// Topology defines the exchange, queue, bindings, and dead-letter setup for a consumer.
type Topology struct {
	Exchange           string
	Queue              string
	BindingKeys        []string
	DeadLetterExchange string
	DeadLetterQueue    string
	Prefetch           int
}

// HandlerFunc processes a single inbound Delivery.
type HandlerFunc func(ctx context.Context, d Delivery)

// Broker is the framework-free messaging abstraction mirroring IMessageBroker in the Node services.
type Broker interface {
	Connect(ctx context.Context) error
	DeclareTopology(top Topology) error
	Consume(ctx context.Context, top Topology, handler HandlerFunc) error
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error
	Close() error
}

