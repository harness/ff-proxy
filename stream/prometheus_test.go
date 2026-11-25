package stream

import (
	"context"
	"testing"

	"github.com/harness-community/sse/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrometheusStream_ObservesEventBytes(t *testing.T) {
	reg := prometheus.NewRegistry()
	next := &mockSubscriber{
		sub: func() (interface{}, error) {
			return &sse.Event{Data: []byte(`{"event":"patch","domain":"flag"}`)}, nil
		},
	}
	ps := NewPrometheusStream("ff_proxy_test_sse", next, reg)

	err := ps.Sub(context.Background(), "topic-a", "", func(id string, v interface{}) error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, testutil.CollectAndCount(ps.eventBytes))

	next.sub = func() (interface{}, error) { return `{"event":"create"}`, nil }
	err = ps.Sub(context.Background(), "topic-b", "", func(id string, v interface{}) error {
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, testutil.CollectAndCount(ps.eventBytes))
}
