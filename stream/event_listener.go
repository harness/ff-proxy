package stream

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/hash"
	"github.com/harness/ff-proxy/log"
)

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

	topic := event.Environment
	content := fmt.Sprintf("event: *\ndata: %s\n\n", event.SSEEvent.Data)

	values := map[StreamEventValue]string{
		StreamEventValueAPIKey: topic,
		StreamEventValueData:   content,
	}

	if err := e.stream.Pub(ctx, topic, NewStreamEvent(values)); err != nil {
		e.log.Error("failed to publish event to stream", "err", err)
		return err
	}
	// TODO - WE PUBLISH ONCE FROM HERE TO THE REDIS STREAM
	e.log.Debug("successfully published event to stream", "topic", topic, "content", content)
	return nil
}
