package ffproxy

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/wings-software/ff-server/pkg/hash"
)

// EventListener implements the golang sdks stream.EventStreamListener interface
// and can be used to hook into the SDK to receive SSE Events that are sent to
// it by the FeatureFlag server.
type EventListener struct {
	log    log.Logger
	stream domain.Stream
	hasher hash.Hasher
}

// NewEventListener creates an EventListener
func NewEventListener(l log.Logger, s domain.Stream, h hash.Hasher) EventListener {
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

	values := map[domain.StreamEventValue]string{
		domain.StreamEventValueAPIKey: topic,
		domain.StreamEventValueData:   content,
	}

	if err := e.stream.Pub(ctx, topic, domain.NewStreamEvent(values)); err != nil {
		e.log.Error("failed to publish event to stream", "err", err)
		return err
	}
	e.log.Debug("successfully published event to stream", "topic", topic, "content", content)
	return nil
}
