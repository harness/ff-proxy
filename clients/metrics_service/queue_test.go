package metricsservice

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/log"
)

func TestQueue_StoreMetrics(t *testing.T) {
	logger := log.NoOpLogger{}

	mr123 := domain.MetricsRequest{
		Size:          7,
		EnvironmentID: "123",
		Metrics: clientgen.Metrics{
			MetricsData: &[]clientgen.MetricsData{
				{
					Attributes:  nil,
					Count:       1,
					MetricsType: "Server",
					Timestamp:   111,
				},
			},
			TargetData: &[]clientgen.TargetData{
				{
					Attributes: nil,
					Identifier: "Foo",
					Name:       "Bar",
				},
			},
		},
	}

	mr456 := domain.MetricsRequest{
		Size:          8,
		EnvironmentID: "456",
		Metrics:       clientgen.Metrics{MetricsData: &[]clientgen.MetricsData{}},
	}

	type args struct {
		metricRequest domain.MetricsRequest
	}

	type expected struct {
		metrics map[string]domain.MetricsRequest
	}

	testCases := map[string]struct {
		args     args
		queue    Queue
		expected expected
	}{
		"Given I call StoreMetrics and we've already exceeded the max queue size": {
			args: args{
				metricRequest: mr123,
			},
			queue: Queue{
				log:   logger,
				queue: make(chan map[string]domain.MetricsRequest, 1), // Buffer so we don't have to run the test concurrently
				metricsData: &metricsMap{
					RWMutex: &sync.RWMutex{},
					metrics: map[string]domain.MetricsRequest{
						mr123.EnvironmentID: mr123,
					},
					ticker:         time.NewTicker(testDuration),
					tickerDuration: testDuration,
					currentSize:    maxEvaluationQueueSize * 2,
				},
				targetData: &metricsMap{
					RWMutex: &sync.RWMutex{},
					metrics: map[string]domain.MetricsRequest{
						mr123.EnvironmentID: mr123,
					},
					ticker:         time.NewTicker(testDuration),
					tickerDuration: testDuration,
					currentSize:    maxTargetQueueSize * 2,
				},
			},
			expected: expected{metrics: map[string]domain.MetricsRequest{
				mr123.EnvironmentID: mr123,
			}},
		},
		"Given I call StoreMetrics and we haven't exceeded the max queue size": {
			args: args{
				metricRequest: mr456,
			},
			queue: Queue{
				log:   logger,
				queue: make(chan map[string]domain.MetricsRequest, 1), // Buffer so we don't have to run the test concurrently
				metricsData: &metricsMap{
					RWMutex: &sync.RWMutex{},
					metrics: map[string]domain.MetricsRequest{
						mr123.EnvironmentID: mr123,
					},
					currentSize: 0,
				},
				targetData: &metricsMap{
					RWMutex:        &sync.RWMutex{},
					metrics:        map[string]domain.MetricsRequest{},
					ticker:         time.NewTicker(testDuration),
					tickerDuration: testDuration,
					currentSize:    0,
				},
			},
			expected: expected{metrics: map[string]domain.MetricsRequest{
				mr123.EnvironmentID: mr123,
				mr456.EnvironmentID: mr456,
			}},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx := context.Background()

			assert.Nil(t, tc.queue.StoreMetrics(ctx, tc.args.metricRequest))

			assert.Equal(t, tc.expected.metrics, tc.queue.metricsData.get())
		})
	}
}

func TestQueue_Listen(t *testing.T) {
	logger := log.NewNoOpLogger()

	mr123 := domain.MetricsRequest{
		Size:          7,
		EnvironmentID: "123",
		Metrics: clientgen.Metrics{
			MetricsData: &[]clientgen.MetricsData{
				{
					Attributes:  nil,
					Count:       1,
					MetricsType: "Server",
					Timestamp:   111,
				},
			},
			TargetData: &[]clientgen.TargetData{
				{
					Attributes: nil,
					Identifier: "Foo",
					Name:       "Bar",
				},
			},
		},
	}

	mr456 := domain.MetricsRequest{
		Size:          8,
		EnvironmentID: "456",
		Metrics:       clientgen.Metrics{MetricsData: &[]clientgen.MetricsData{}},
	}

	type args struct {
		metricsRequests []domain.MetricsRequest
		flushDuration   time.Duration
	}

	type expected struct {
		eventCount  int
		metricsData map[string]domain.MetricsRequest
	}

	testCases := map[string]struct {
		args     args
		expected expected
	}{
		"Given I have a queue, I add metrics requests to it and the flush interval expires": {
			args: args{
				metricsRequests: []domain.MetricsRequest{mr123, mr456},
				flushDuration:   5 * time.Second,
			},
			expected: expected{
				metricsData: map[string]domain.MetricsRequest{
					mr123.EnvironmentID: mr123,
					mr456.EnvironmentID: mr456,
				},
				eventCount: 1,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.args.flushDuration*3)
			defer cancel()

			q := NewQueue(ctx, logger, tc.args.flushDuration)

			go func() {
				for _, mr := range tc.args.metricsRequests {
					_ = q.StoreMetrics(ctx, mr)
				}
			}()

			actual := map[string]domain.MetricsRequest{}
			eventCount := 0

			for mr := range q.Listen(ctx) {
				eventCount++

				for k, v := range mr {
					actual[k] = v
				}

				if eventCount == tc.expected.eventCount {
					cancel()
				}
			}

			assert.Equal(t, tc.expected.metricsData, actual)
		})
	}
}
