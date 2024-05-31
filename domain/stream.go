package domain

import (
	"context"
)

// Stream defines the Stream interface
type Stream interface {
	Publisher
	Subscriber
}

// Publisher defines the interface for publishing to a stream
type Publisher interface {
	Pub(ctx context.Context, channel string, value interface{}) error
	Close(channel string) error
}

// Subscriber defines the interface for subscribing to a stream
type Subscriber interface {
	Sub(ctx context.Context, channel string, id string, message HandleMessageFn) error
}

// Closer defines the interface for closing a stream
type Closer interface {
	Close(topic string) error
}

// HandleMessageFn is the function that gets called whenever a subscriber receives a message on a stream
type HandleMessageFn func(id string, v interface{}) error
