package stream

import (
	"context"
	"errors"
	"fmt"

	"github.com/fanout/go-gripcontrol"
	"github.com/fanout/go-pubcontrol"
	"github.com/harness/ff-golang-server-sdk/stream"
	"github.com/harness/ff-proxy/token"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/harness/ff-proxy/log"
)

// GripStream is the interface for publishing events to a grip channel
type GripStream interface {
	// PublishHttpStream publishes an HTTP stream format message to all of the configured PubControlClients
	// with a specified channel, message, and optional ID, previous ID, and callback.
	// Note that the 'http_stream' parameter can be provided as either an HttpStreamFormat
	// instance or a string / byte array (in which case an HttpStreamFormat instance will
	// automatically be created and have the 'content' field set to the specified value).
	PublishHttpStream(channel string, content interface{}, id string, prevID string) error

	Publish(channel string, item *pubcontrol.Item) error
}

type streamEvent struct {
	channel string // channel is the grip channel that we want to forward the event to
	content string // content is the data that we want to publish to the channel
	err     error
}

// StreamWorker is the type that subscribes to the SSEEvent Stream that the EventListener
// forwards SSEEvents from the embedded SDKs to and forwards them on to clients
type StreamWorker struct {
	log        log.Logger
	gpc        GripStream
	ssePublish *prometheus.CounterVec
	sseClose   *prometheus.CounterVec
}

// NewStreamWorker creates a StreamWorker
func NewStreamWorker(l log.Logger, gpc GripStream, reg *prometheus.Registry) StreamWorker {
	l = l.With("component", "StreamWorker")
	s := StreamWorker{
		log: l,
		gpc: gpc,
		ssePublish: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ff_proxy_sse_publish",
			Help: "Records the number of sse events the proxy has received and forwarded on to clients",
		},
			[]string{"environment", "api_key", "error"},
		),
		sseClose: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ff_proxy_sse_stream_close",
			Help: "Records the number of times the proxy closes an SSE stream",
		},
			[]string{"environment", "api_key", "error"},
		),
	}

	reg.MustRegister(s.ssePublish, s.sseClose)
	return s
}

// Pub makes StreamWorker implement the golang sdks stream.EventStreamListener
// interface.
func (s StreamWorker) Pub(ctx context.Context, event stream.Event) (err error) {
	if errors.Is(event.Err, stream.ErrStreamDisconnect) {
		return s.closeStream(event)
	}

	if event.SSEEvent == nil {
		return errors.New("can't publish event with nil SSEEvent")
	}

	defer func() {
		errLabel := "false"
		if err != nil {
			errLabel = "true"
		}

		s.ssePublish.WithLabelValues(event.Environment, token.MaskRight(event.APIKey), errLabel).Inc()
	}()

	topic := event.Environment
	content := fmt.Sprintf("event: *\ndata: %s\n\n", event.SSEEvent.Data)

	if err := s.publish(ctx, streamEvent{
		channel: topic,
		content: content,
		err:     nil,
	}); err != nil {
		s.log.Error("stream worker failed", "topic", topic)
	}

	return nil
}

// publish publishes events to the GripStream
func (s StreamWorker) publish(ctx context.Context, f streamEvent) error {
	return s.gpc.PublishHttpStream(f.channel, f.content, "", "")
}

func (s StreamWorker) closeStream(event stream.Event) error {
	item := pubcontrol.NewItem([]pubcontrol.Formatter{&gripcontrol.HttpStreamFormat{Close: true}}, "", "")

	if err := s.gpc.Publish(event.Environment, item); err != nil {
		s.log.Error("failed to close stream", "name", event.Environment, "err", err)
		s.sseClose.WithLabelValues(event.Environment, token.MaskRight(event.APIKey), "true")
		return err
	}

	s.sseClose.WithLabelValues(event.Environment, token.MaskRight(event.APIKey), "false")
	return nil
}
