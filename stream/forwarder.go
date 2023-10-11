package stream

import (
	"context"

	"github.com/harness/ff-proxy/v2/domain"

	"github.com/harness/ff-proxy/v2/log"
)

// Forwarder is a type that can be used to handle messages from a stream and
// forward them on to another stream
type Forwarder struct {
	log    log.Logger
	next   domain.MessageHandler
	stream Publisher
}

// NewForwarder creates a Forwarder
func NewForwarder(l log.Logger, stream Publisher, next domain.MessageHandler) Forwarder {
	l = l.With("component", "Forwarder")
	return Forwarder{
		log:    l,
		next:   next,
		stream: stream,
	}
}

// HandleMessage makes Forwarder implement the MessageHandler interface. It calls the decorated
// MessageHandler and once that's returned forwards it on to the next stream.
func (s Forwarder) HandleMessage(ctx context.Context, msg domain.SSEMessage) (err error) {
	defer func() {
		// If we get an error handling the message we probably don't want to forward it on
		if err != nil {
			return
		}

		// Flag and TargetSegment change messages are the only ones we need to care about
		// forwarding on to the read replica Proxy or SDKs
		if msg.Domain != domain.MsgDomainFeature && msg.Domain != domain.MsgDomainSegment {
			return
		}

		topic := msg.Environment

		if err := s.stream.Pub(ctx, topic, msg); err != nil {
			s.log.Error("failed to forward SSEEvent to channel=%s: %s", "", err)
		}
	}()

	return s.next.HandleMessage(ctx, msg)
}
