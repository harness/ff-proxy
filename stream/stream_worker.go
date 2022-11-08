package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/harness/ff-golang-server-sdk/stream"

	"github.com/harness/ff-proxy/log"
)

// GripStream is the interface for publishing events to a grip channel
type GripStream interface {
	// Publish an HTTP stream format message to all of the configured PubControlClients
	// with a specified channel, message, and optional ID, previous ID, and callback.
	// Note that the 'http_stream' parameter can be provided as either an HttpStreamFormat
	// instance or a string / byte array (in which case an HttpStreamFormat instance will
	// automatically be created and have the 'content' field set to the specified value).
	PublishHttpStream(channel string, content interface{}, id string, prevID string) error
}

type streamEvent struct {
	channel string // channel is the grip channel that we want to forward the event to
	content string // content is the data that we want to publish to the channle
	err     error
}

// StreamWorker is the type that subscribes to the SSEEvent Stream that the EventListener
// forwards SSEEvents from the embedded SDKs to and forwards them on to clients
type StreamWorker struct {
	log log.Logger
	gpc GripStream
}

// NewStreamWorker creates a StreamWorker
func NewStreamWorker(l log.Logger, gpc GripStream) StreamWorker {
	l = l.With("component", "StreamWorker")
	return StreamWorker{
		log: l,
		gpc: gpc,
	}
}

// Pub makes StreamWorker implement the golang sdks stream.EventStreamListener
// interface.
func (s StreamWorker) Pub(ctx context.Context, event stream.Event) error {
	if event.SSEEvent == nil {
		return errors.New("can't publish event with nil SSEEvent")
	}

	topic := event.Environment
	content := fmt.Sprintf("event: *\ndata: %s\n\n", event.SSEEvent.Data)

	if err := s.publish(ctx, streamEvent{
		channel: topic,
		content: content,
		err:     nil,
	}); err != nil {
		s.log.Error("stream worker failed", "topic", topic)
		// We hit an error, sleep and try subscribing again
		time.Sleep(10 * time.Second)
	}

	return nil
}

// publish publishes events to the GripStream
func (s StreamWorker) publish(ctx context.Context, f streamEvent) error {
	return s.gpc.PublishHttpStream(f.channel, f.content, "", "")
}
