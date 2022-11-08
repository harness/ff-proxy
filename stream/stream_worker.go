package stream

import (
	"context"
	"time"

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
	log    log.Logger
	gpc    GripStream
	stream Stream
	topics []string
}

// NewStreamWorker creates a StreamWorker
func NewStreamWorker(l log.Logger, gpc GripStream, stream Stream, topics ...string) StreamWorker {
	l = l.With("component", "StreamWorker")
	return StreamWorker{
		log:    l,
		gpc:    gpc,
		stream: stream,
		topics: topics,
	}
}

// Run starts the stream workers process of listening for events from the passed
// Stream and forwarding them on to the passed GripStream. Run starts a goroutine
// for each topic passed to the StreamWorker so that events for each topic are
// processed in parallel. It will run until the context has been cancelled and
// in the event of an error it will sleep and try again.
func (s StreamWorker) Run(ctx context.Context) {
	for _, topic := range s.topics {
		s.log.Info("starting stream worker", "topic", topic)
		go func(topic string) {
			for {
				select {
				case <-ctx.Done():
					s.log.Info("stopping stream worker", "err", ctx.Err())
					return
				default:
					if err := s.processTopic(ctx, topic); err != nil {
						if err == context.Canceled {
							return
						}
						s.log.Error("stream worker failed", "topic", topic)
						// We hit an error, sleep and try subscribing again
						time.Sleep(10 * time.Second)
					}

					// If for some reason run exits without an error we just
					// re-enter the loop and run again
				}
			}
		}(topic)
	}
}

// processTopic performs the logic of listening for events on a Stream for
// a topic and forwarding them on to the GripStream for that topic
func (s StreamWorker) processTopic(ctx context.Context, topic string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events := s.subscribe(ctx, topic)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e, ok := <-events:
			if !ok {
				return nil
			}

			if e.err != nil {
				return e.err
			}

			if err := s.publish(ctx, e); err != nil {
				return err
			}
			s.log.Debug("succesfully published event to pushpin", "topic", topic, "channel", e.channel, "content", e.content)
		}
	}
}

// subscribe will subscribe to a topic on the Stream that was passed to the worker
// and write any events to a channel
func (s StreamWorker) subscribe(ctx context.Context, topic string) <-chan streamEvent {
	out := make(chan streamEvent)
	go func() {
		defer func() {
			close(out)
		}()

		err := s.stream.Sub(ctx, topic, "", func(event StreamEvent) {
			apiKey := event.Values[StreamEventValueAPIKey]
			content := event.Values[StreamEventValueData]

			select {
			case <-ctx.Done():
				return
			case out <- streamEvent{channel: apiKey, content: content}:
			}
		})
		if err != nil {
			out <- streamEvent{err: err}
			return
		}
	}()
	return out
}

// publish publishes events to the GripStream
func (s StreamWorker) publish(ctx context.Context, f streamEvent) error {
	return s.gpc.PublishHttpStream(f.channel, f.content, "", "")
}
