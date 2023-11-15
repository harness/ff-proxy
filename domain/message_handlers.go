package domain

import (
	"context"
	"io"
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

// ReadReplicaMessageHandler defines the message handler used by the read replica.
// The ReadReplica doesn't need to care about 99% of the messages it receives, and
// the only thing it really needs to do is forward these messages on to any connected
// SDKs. However, if the 'Writer' Proxy connects/disconnects from the Harness SaaS stream,
// it sends a message to the read replica(s) to let them know about this event.
//
// The Replica can then use these events to forcibly disconnect SDKs and block new stream
// requests until the Writer Proxy -> SaaS stream has been reestablished
type ReadReplicaMessageHandler struct {
}

// NewReadReplicaMessageHandler creates a ReadReplicaMessageHandler
func NewReadReplicaMessageHandler() ReadReplicaMessageHandler {
	return ReadReplicaMessageHandler{}
}

// HandleMessage makes ReadReplicaMessageHandler implement the MessageHandler interface.
// It checks the message's event type & domain and calls the appropriate method to deal with these.
func (r ReadReplicaMessageHandler) HandleMessage(_ context.Context, msg SSEMessage) error {
	// Any other event types we don't care about, we just want our chain of message handlers
	// to forward this on to pushpin so SDKs get these events.
	if msg.Event != "stream_action" {
		// Return EOF to indicate the stream was closed
		if msg.Domain == "disconnect" {
			return io.EOF
		}
		return nil
	}

	if msg.Event == "environmentsRemoved" || msg.Event == "apiKeyRemoved" {
		return io.EOF
	}

	return nil
}
