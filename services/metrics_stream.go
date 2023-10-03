package services

import (
	"context"

	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/stream"
)

const (
	SDKMetricsStream = "stream:sdk_metrics"
)

// MetricsStream is a type for publishing metrics to a stream and implements the MetricService interface
type MetricsStream struct {
	stream  stream.Publisher
	channel string
}

// NewMetricsStream creates a metrics stream
func NewMetricsStream(s stream.Publisher) MetricsStream {
	return MetricsStream{
		stream:  s,
		channel: "stream:sdk_metrics",
	}
}

// StoreMetrics pushes metrics onto a stream
func (m MetricsStream) StoreMetrics(ctx context.Context, req domain.MetricsRequest) error {
	return m.stream.Pub(ctx, m.channel, &req)
}
