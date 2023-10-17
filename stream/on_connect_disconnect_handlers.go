package stream

import (
	"context"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

// pollFn defines the function that polls Harness SaaS for changes
type pollFn func() error

// getConnectedStreamsFn defines the function that returns the names of open streams between the Proxy & SDKs
type getConnectedStreamsFn func() map[string]interface{}

// SaasStreamOnDisconnect is called anytime we disconnect or fail to reconnect to the SaaS SSE stream and does the following
// - Sets the status of the SaaS stream in the cache to unhealthy, this means any new /stream requests to writer or read proxy's will be rejects
// - Polls saas for the latest config and refreshes the cache with any changes
// - Closes any 'Write Replica' Proxy -> SDK streams
// - Notifies 'read replica' proxy's that there's been a disconnection between the 'Write replica' and SaaS
func SaasStreamOnDisconnect(l log.Logger, streamHealth Health, pp Pushpin, redisSSEStream Stream, streams getConnectedStreamsFn, pollFn pollFn) func() {
	return func() {
		l.Info("disconnected from Harness SaaS SSE Stream")

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		// Set to false so the ProxyService will reject any /stream requests from SDKs until we've reconnected
		_ = streamHealth.SetUnhealthy(ctx)

		// Poll latest config from SaaS, this is to make sure we don't miss any changes that could have
		// happened while the stream was disconnected
		if err := pollFn(); err != nil {
			l.Error("SSE stream disconnected, failed to poll for new config", "err", err)
		} else {
			l.Info("successfully polled Harness SaaS for changes")
		}

		// Close any open stream between this Proxy and SDKs. This is to force SDKs to poll the Proxy for
		// changes until we've a healthy SaaS -> Proxy stream to make sure they don't miss out on changes
		// the Proxy may have pulled down while the Proxy -> Saas stream was down.
		for streamID := range streams() {
			if err := pp.CloseStream(streamID); err != nil {
				l.Error("failed to close Proxy->SDK stream", "streamID", streamID, "err", err)
			}
		}

		// Publish an event to the redis stream that the read replica proxy's are listening on to let them
		// know we've disconnected from SaaS.
		if err := redisSSEStream.Publish(ctx, domain.SSEMessage{Event: "stream_action", Domain: "disconnect"}); err != nil {
			l.Error("failed to publish stream disconnect message to redis", "err", err)
		}
	}
}

// SaasStreamOnConnect sets the status of the SaaS stream to healthy in the cache
func SaasStreamOnConnect(l log.Logger, streamHealth Health) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		l.Info("connected to Harness SaaS SSE Stream")
		if err := streamHealth.SetHealthy(ctx); err != nil {
			l.Error("failed to update SaaS stream status in cache", "err", err)
		}
	}
}

// ReadReplicaSSEStreamOnDisconnect closes any open 'Read Replica' Proxy -> SDK streams
func ReadReplicaSSEStreamOnDisconnect(l log.Logger, pp Pushpin, streams getConnectedStreamsFn) func() {
	return func() {
		// Close any open stream between this Proxy and SDKs. This is to force SDKs to poll the Proxy for
		// changes until we've a healthy SaaS -> Proxy stream to make sure they don't miss out on changes
		// the Proxy may have pulled down while the Proxy -> Saas stream was down.
		for streamID := range streams() {
			if err := pp.CloseStream(streamID); err != nil {
				l.Error("failed to close Proxy->SDK stream", "streamID", streamID, "err", err)
			}
		}
	}
}
