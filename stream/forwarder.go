package stream

import (
	"context"

	"github.com/harness/ff-proxy/v2/domain"

	"github.com/harness/ff-proxy/v2/log"
)

// WithStreamName is an optional func for configuring the name of the Stream we want
// to forward the message on to
func WithStreamName(name string) func(*Forwarder) {
	return func(f *Forwarder) {
		f.streamName = name
	}
}

// Forwarder is a type that can be used to handle messages from a stream and
// forward them on to another stream
type Forwarder struct {
	log        log.Logger
	next       domain.MessageHandler
	streamName string
	stream     domain.Publisher
}

// NewForwarder creates a Forwarder
func NewForwarder(l log.Logger, stream domain.Publisher, next domain.MessageHandler, options ...func(*Forwarder)) Forwarder {
	l = l.With("component", "Forwarder")
	f := &Forwarder{
		log:    l,
		next:   next,
		stream: stream,
	}

	for _, opt := range options {
		opt(f)
	}
	return *f
}

// HandleMessage makes Forwarder implement the MessageHandler interface. It calls the decorated
// MessageHandler and once that's returned forwards it on to the next stream.
func (s Forwarder) HandleMessage(ctx context.Context, msg domain.SSEMessage) (err error) {
	defer func() {
		// If we get an error handling the message we probably don't want to forward it on
		if err != nil {
			return
		}

		topic := s.streamName
		if topic == "" {
			topic = msg.Environment
		}

		// Send close stream msg if required
		s.closeStream(msg)

		// Flag and TargetSegment change messages are the only ones we need to care about
		// forwarding on to the read replica Proxy or SDKs
		if msg.Domain != domain.MsgDomainFeature &&
			msg.Domain != domain.MsgDomainSegment &&
			msg.Event != domain.EventEnvironmentRemoved &&
			msg.Event != domain.EventAPIKeyRemoved {
			return
		}

		if err = s.stream.Pub(ctx, topic, msg); err != nil {
			s.log.Error("failed to forward event to channel=%s: %s", "", err)
		}
	}()

	return s.next.HandleMessage(ctx, msg)
}

func (s Forwarder) closeStream(msg domain.SSEMessage) {
	if msg.Event == domain.EventEnvironmentRemoved || msg.Event == domain.EventAPIKeyRemoved {
		// if the key or api key has been deleted we want to close the stream.
		for _, v := range msg.Environments {
			_ = s.stream.Close(v)
		}
	}

}
