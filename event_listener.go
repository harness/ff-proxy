package ffproxy

import (
	"context"
	"errors"

	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/log"
	"github.com/wings-software/ff-server/pkg/hash"
)

// Stream is a placeholder interface that will be implemented by our InMemStream
// and Redis Stream (FFM-2105, FFM-2102)
type Stream interface {
	Pub(ctx context.Context, topic string, values map[string]interface{}) error
}

// EventListener implements the golang sdks stream.EventStreamListener interface
// and can be used to hook into the SDK to receive SSE Events that are sent to
// it by the FeatureFlag server.
type EventListener struct {
	log    log.Logger
	stream Stream
	hasher hash.Hasher
}

// NewEventListener creates an EventListener
func NewEventListener(l log.Logger, s Stream, h hash.Hasher) EventListener {
	l = l.With("component", "EventListener")
	return EventListener{
		log:    l,
		stream: s,
		hasher: h,
	}
}

// Pub makes EventListener implement the golang sdks stream.EventStreamListener
// interface.
func (e EventListener) Pub(ctx context.Context, event stream.Event) error {
	if event.SSEEvent == nil {
		return errors.New("can't publish event with nil SSEEvent")
	}

	topic := e.hasher.Hash(event.APIKey)
	values := map[string]interface{}{
		"ID":    event.SSEEvent.ID,
		"Data":  event.SSEEvent.Data,
		"Event": event.SSEEvent.Event,
		"Retry": event.SSEEvent.Retry,
	}

	if err := e.stream.Pub(ctx, topic, values); err != nil {
		e.log.Error("failed to publish event to stream", "err", err)
		return err
	}
	return nil
}
