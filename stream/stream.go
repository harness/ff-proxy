package stream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/harness-community/sse/v3"
	jsoniter "github.com/json-iterator/go"
	"gopkg.in/cenkalti/backoff.v1"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

var (
	// ErrPublishing is the error returned when we fail to publish an event to a stream
	ErrPublishing = errors.New("failed to publish to stream")

	ErrSubscribing = errors.New("failed to subscribe to stream")

	errParsingMessage = errors.New("errParsingMessage")
)

func WithOnConnect(fn func()) func(s *Stream) {
	return func(s *Stream) {
		s.onConnect = fn
	}
}

// WithOnDisconnect is an optional func for setting the onDisconnect field
func WithOnDisconnect(fn func()) func(s *Stream) {
	return func(s *Stream) {
		s.onDisconnect = fn
	}
}

// WithBackoff is an optional func for seeting the backoff duration
func WithBackoff(b backoff.BackOff) func(s *Stream) {
	return func(s *Stream) {
		s.backoff = b
	}
}

// Stream defines a type that can subscribe to a stream and handle events that come off it
type Stream struct {
	log            log.Logger
	topic          string
	stream         domain.Stream
	messageHandler domain.MessageHandler
	onDisconnect   func()
	onConnect      func()
	backoff        backoff.BackOff
}

// NewStream opens a subscription to the client service's stream endpoint
func NewStream(l log.Logger, topic string, s domain.Stream, m domain.MessageHandler, options ...func(s *Stream)) Stream {
	l = l.With("component", "Stream", "topic", topic)
	stream := &Stream{
		log:            l,
		topic:          topic,
		stream:         s,
		messageHandler: m,
	}

	for _, opt := range options {
		opt(stream)
	}

	if stream.backoff == nil {
		stream.backoff = backoff.NewConstantBackOff(1 * time.Minute)
	}
	return *stream
}

// Publish publishes a message to the stream.
func (s Stream) Publish(ctx context.Context, msg interface{}) (err error) {
	return s.stream.Pub(ctx, s.topic, msg)
}

// Subscribe connects to the stream and registers a handler to handle events coming off the stream.
// If the stream disconnects backoff and attempt to subscribe again.
func (s Stream) Subscribe(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				s.subscribe(ctx)
			}
		}
	}()
}

func (s Stream) subscribe(ctx context.Context) {
	if s.onConnect != nil {
		s.onConnect()
	}

	msgID := ""
	err := s.stream.Sub(ctx, s.topic, msgID, func(id string, v interface{}) (err error) {
		msg, err := parseMessage(v)
		if err != nil {
			return err
		}

		msgID = id

		return s.messageHandler.HandleMessage(ctx, msg)
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
	}

	if s.onDisconnect != nil {
		s.onDisconnect()
	}

	backoffDuration := s.backoff.NextBackOff()
	s.log.Warn("disconnected from stream, backing off and retrying", "backoff_duration", backoffDuration, "err", err, "msgID", msgID)
	time.Sleep(backoffDuration)
}

// This logic should maybe move to be inside the specific messageHandler implementations, that way we could
// keep the Stream type completely generic/reusable for different types of messages instead of constantly
// having to update this to decode different message formats
func parseMessage(v interface{}) (domain.SSEMessage, error) {
	if s, ok := v.(string); ok {
		m := domain.SSEMessage{}
		if err := jsoniter.Unmarshal([]byte(s), &m); err != nil {
			return m, fmt.Errorf("%w: %s", errParsingMessage, err)
		}
		return m, nil
	}

	event, ok := v.(*sse.Event)
	if !ok {
		return domain.SSEMessage{}, fmt.Errorf("%w: expected message type to be *sse.Event, got=%T", errParsingMessage, v)
	}

	m := domain.SSEMessage{}
	if err := jsoniter.Unmarshal(event.Data, &m); err != nil {
		return domain.SSEMessage{}, fmt.Errorf("%w: %s", errParsingMessage, err)
	}

	return m, nil
}
