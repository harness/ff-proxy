package metricsservice

import (
	"context"

	"github.com/harness/ff-proxy/v2/domain"
)

const (
	// SDKMetricsStream is the name of the stream read replica proxy's write metrics to
	SDKMetricsStream = "stream:sdk_metrics"
)

// Stream is a type for publishing metrics to a stream and implements the MetricService interface
type Stream struct {
	stream  domain.Publisher
	channel string
}

// NewStream creates a metrics stream
func NewStream(s domain.Publisher) Stream {
	return Stream{
		stream:  s,
		channel: "stream:sdk_metrics",
	}
}

// StoreMetrics pushes metrics onto a stream
func (m Stream) StoreMetrics(ctx context.Context, req domain.MetricsRequest) error {
	return m.stream.Pub(ctx, m.channel, &req)
}
