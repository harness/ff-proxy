package stream

import (
	"context"
	"errors"
)

var (
	// ErrPublishing is the error returned when we fail to publish an event to a stream
	ErrPublishing = errors.New("failed to publish to stream")

	ErrSubscribing = errors.New("failed to subscribe to stream")
)

// Stream defines the stream interface
type Stream interface {
	Publisher
	Subscriber
}

// Publisher defines the interface for publishing to a stream
type Publisher interface {
	Pub(ctx context.Context, channel string, value interface{}) error
}

// Subscriber defines the interface for subscribing to a stream
type Subscriber interface {
	Sub(ctx context.Context, channel string, id string, message HandleMessageFn) error
}

// HandleMessageFn is the function that gets called whenever a subscriber receives a message on a stream
type HandleMessageFn func(id string, v interface{}) error
