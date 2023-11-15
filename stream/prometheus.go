package stream

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/harness/ff-proxy/v2/domain"
)

// PrometheusStream is a Stream decorator for recording prometheus metrics around the number
// of messages received and published to a stream
type PrometheusStream struct {
	next              domain.Stream
	messagesPublished *prometheus.CounterVec
	messagesReceived  *prometheus.CounterVec
}

func (p *PrometheusStream) Close(channel string) error {
	return p.next.Close(channel)
}

// NewPrometheusStream creates a PrometheusStream
func NewPrometheusStream(name string, next domain.Stream, reg *prometheus.Registry) *PrometheusStream {
	p := &PrometheusStream{
		next: next,
		messagesPublished: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("%s_messages_published", name),
			Help: "Records the number of messages published to the stream",
		},
			[]string{"topic", "error"},
		),
		messagesReceived: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: fmt.Sprintf("%s_messages_received", name),
			Help: "Records the number of messages received on the stream",
		},
			[]string{"topic", "error"},
		),
	}

	reg.MustRegister(p.messagesPublished, p.messagesReceived)
	return p
}

// Pub calls the decorated streams Pub method and increments a prometheus metric for each message published
func (p *PrometheusStream) Pub(ctx context.Context, channel string, msg interface{}) (err error) {
	defer func() {
		errLabel := parseErrorLabel(err)
		p.messagesPublished.WithLabelValues(channel, errLabel).Inc()
	}()

	return p.next.Pub(ctx, channel, msg)
}

// Sub calls the decorated streams Sub method and increments a prometheus metric for each message received
func (p *PrometheusStream) Sub(ctx context.Context, channel string, id string, msgFn domain.HandleMessageFn) error {
	return p.next.Sub(ctx, channel, id, func(id string, v interface{}) error {
		err := msgFn(id, v)
		errLabel := parseErrorLabel(err)

		p.messagesReceived.WithLabelValues(channel, errLabel).Inc()
		return err
	})
}

func parseErrorLabel(err error) string {
	if err == nil {
		return "false"
	}

	switch {
	case errors.Is(err, io.EOF):
		return "stream_disconnect"
	case errors.Is(err, ErrPublishing):
		return "err_publishing"
	case errors.Is(err, ErrSubscribing):
		return "err_subscribing"
	case errors.Is(err, errParsingMessage):
		return "err_parsing_message"
	default:
		return "unexpected_error"
	}
}
