package stream

import (
	"context"
	"errors"
)

var (
	// ErrPublishing is the error returned when we fail to publish an event to a stream
	ErrPublishing = errors.New("failed to publish to stream")
)

// Stream defines the stream interface
type Stream interface {
	Pub(ctx context.Context, channel string, value interface{}) error
}
