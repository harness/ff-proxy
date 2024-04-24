package domain

import (
	"context"
	"io"

	"github.com/harness/ff-proxy/v2/log"
)

// MessageHandler defines the interface for handling an SSE message
type MessageHandler interface {
	HandleMessage(ctx context.Context, m SSEMessage) error
}

// NoOpMessageHandler is a message handler that does nothing
type NoOpMessageHandler struct {
}

// HandleMessage makes NoOpMessageHandler implement the MessageHandler interface
func (n NoOpMessageHandler) HandleMessage(_ context.Context, _ SSEMessage) error {
	return nil
}

type healther interface {
	SetUnhealthy(ctx context.Context) error
	SetHealthy(ctx context.Context) error
}

// ReadReplicaMessageHandler defines the message handler used by the read replica.
// The ReadReplica doesn't need to care about 99% of the messages it receives, and
// the only thing it really needs to do is forward these messages on to any connected
// SDKs. However, if the 'Writer' Proxy connects/disconnects from the Harness SaaS stream,
// it sends a message to the read replica(s) to let them know about this event.
//
// The Replica can then use these events to forcibly disconnect SDKs and block new stream
// requests until the Writer Proxy -> SaaS stream has been reestablished
type ReadReplicaMessageHandler struct {
	log          log.Logger
	streamStatus healther
}

// NewReadReplicaMessageHandler creates a ReadReplicaMessageHandler
func NewReadReplicaMessageHandler(l log.Logger, s healther) ReadReplicaMessageHandler {
	l = l.With("component", "ReadReplicaMessageHandler")
	return ReadReplicaMessageHandler{
		log:          l,
		streamStatus: s,
	}
}

// HandleMessage makes ReadReplicaMessageHandler implement the MessageHandler interface.
// It checks the message's event type & domain and calls the appropriate method to deal with these.
func (r ReadReplicaMessageHandler) HandleMessage(ctx context.Context, msg SSEMessage) error {
	if msg.Event == "stream_action" {
		return r.handleStreamAction(ctx, msg)
	}

	if msg.Event == "environmentsRemoved" || msg.Event == "apiKeyRemoved" {
		return io.EOF
	}
	return nil

}

// handleStreamAction sets the internal StreamHealth in the read replica based on the type of message we get
func (r ReadReplicaMessageHandler) handleStreamAction(ctx context.Context, msg SSEMessage) error {
	if msg.Domain == "disconnect" {
		r.log.Info("received stream disconnect event from primary proxy")

		if err := r.streamStatus.SetUnhealthy(ctx); err != nil {
			r.log.Error("failed to set unhealthy stream status", "err", err)
		}

		// Return EOF to indicate the stream was closed
		return io.EOF
	}

	if msg.Domain == "connect" {
		r.log.Info("received stream connect event from primary proxy")

		if err := r.streamStatus.SetHealthy(ctx); err != nil {
			r.log.Error("failed to set healthy stream status", "err", err)
		}
	}

	return nil
}
