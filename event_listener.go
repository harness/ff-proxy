package ffproxy

import (
	"context"
	"fmt"

	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/log"
	"github.com/wings-software/ff-server/pkg/hash"
)

// Stream is a placeholder interface that will be implemented by our InMemStream
// and Redis Stream (FFM-2105, FFM-2102)
type Stream interface {
	Pub(ctx context.Context, topic string, event interface{})
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
	e.log.Info("got Event from SDK", "event", fmt.Sprintf("%+v", event))

	// TODO:Push event to stream implementation
	//topic := e.hasher.Hash(event.APIKey)
	//e.stream.Pub(ctx, topic, event.SSEEvent)

	return nil
}
