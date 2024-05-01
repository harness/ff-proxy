package metricsservice

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/log"
)

type mockRedisStream struct {
	*sync.RWMutex
	idx    int
	events []string
}

func newMockRedisStream(events ...string) *mockRedisStream {
	return &mockRedisStream{
		RWMutex: &sync.RWMutex{},
		idx:     0,
		events:  events,
	}
}

func (m *mockRedisStream) Sub(ctx context.Context, channel string, id string, messageFn domain.HandleMessageFn) error {
	m.Lock()
	defer func() {
		m.Unlock()
		m.idx++
	}()

	if m.idx >= len(m.events) {
		// Worker subscribe func will only exit if we return context.Canceled error
		return context.Canceled
	}

	return messageFn(id, m.events[m.idx])
}

type mockMetricsService struct {
	metrics chan domain.MetricsRequest
}

func (m *mockMetricsService) PostMetrics(ctx context.Context, envID string, r domain.MetricsRequest, clusterIdentifier string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case m.metrics <- r:
	}

	return nil
}

func (m *mockMetricsService) listen() <-chan domain.MetricsRequest {
	return m.metrics
}

func mustMarshalToString(mr ...domain.MetricsRequest) []string {
	ss := []string{}

	for _, m := range mr {
		b, err := json.Marshal(m)
		if err != nil {
			panic(err)
		}

		ss = append(ss, string(b))
	}

	return ss
}

func TestWorker_Start(t *testing.T) {
	mr123 := domain.MetricsRequest{
		Size:          176,
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
		Size:          41,
		EnvironmentID: "456",
		Metrics:       clientgen.Metrics{MetricsData: &[]clientgen.MetricsData{}},
	}

	mr123EvaluationMetricsExpected := domain.MetricsRequest{
		Size:          176,
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
			TargetData: nil,
		},
	}

	type args struct {
	}

	type mocks struct {
		redisStream   *mockRedisStream
		metricService *mockMetricsService
		store         Queue
	}

	type expected struct {
		metrics []domain.MetricsRequest
	}

	testCases := map[string]struct {
		args     args
		mocks    mocks
		expected expected
	}{
		"Given I have a redis stream with one metrics request on it": {
			mocks: mocks{
				redisStream: newMockRedisStream(mustMarshalToString(mr123)...),
				metricService: &mockMetricsService{
					metrics: make(chan domain.MetricsRequest),
				},
			},
			expected: expected{metrics: []domain.MetricsRequest{mr123EvaluationMetricsExpected}},
		},
		"Given I have a redis stream with two metrics requests on it": {
			mocks: mocks{
				redisStream: newMockRedisStream(mustMarshalToString(mr123, mr456)...),
				metricService: &mockMetricsService{
					metrics: make(chan domain.MetricsRequest),
				},
			},
			expected: expected{metrics: []domain.MetricsRequest{mr123EvaluationMetricsExpected, mr456}},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			queue := NewQueue(ctx, log.NoOpLogger{}, 5*time.Second)
			w := NewWorker(log.NoOpLogger{}, queue, tc.mocks.metricService, tc.mocks.redisStream, 1, "1")

			w.Start(ctx)

			actual := []domain.MetricsRequest{}

			// Wait for metrics to be 'posted' to the metrics service
			done := false
			for !done {
				m, ok := <-tc.mocks.metricService.listen()
				if !ok {
					done = true
					continue
				}

				actual = append(actual, m)

				if len(actual) == len(tc.expected.metrics) {
					done = true
				}
			}

			for i := 0; i < len(tc.expected.metrics); i++ {
				exp := tc.expected.metrics[i]
				act := actual[i]

				assert.Equal(t, exp.EnvironmentID, act.EnvironmentID)

				if exp.MetricsData != nil {
					for j := 0; j < len(*exp.MetricsData); j++ {
						expCopy := *exp.MetricsData
						actCopy := *act.MetricsData

						expMD := expCopy[j]
						actMD := actCopy[j]

						assert.Equal(t, expMD.Count, actMD.Count)
						assert.Equal(t, expMD.Attributes, actMD.Attributes)
						assert.Equal(t, expMD.MetricsType, actMD.MetricsType)
					}
				}

				if exp.TargetData != nil {
					for j := 0; j < len(*exp.TargetData); j++ {
						expCopy := *exp.TargetData
						actCopy := *act.TargetData

						expTD := expCopy[j]
						actTD := actCopy[j]

						assert.Equal(t, expTD.Name, actTD.Name)
						assert.Equal(t, expTD.Identifier, actTD.Identifier)
						assert.Equal(t, expTD.Attributes, actTD.Attributes)
					}
				}
			}
		})
	}
}

// Benchmarking the handle metrics function reading from a channel vs just taking a raw bytes
func Benchmark_Start(b *testing.B) {
	benchmarks := []struct {
		name          string
		metricsEvents int
		concurrency   int
		startFn       func(n int, c int)
	}{
		{
			name:          "StartOne- 10000 metrics, concurrency=1",
			metricsEvents: 10000,
			concurrency:   1,
			startFn:       StartOne,
		},
		{
			name:          "StartTwo - 10000 metrics, concurrency=1",
			metricsEvents: 10000,
			concurrency:   1,
			startFn:       StartTwo,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bm.startFn(bm.metricsEvents, bm.concurrency)
			}
		})
	}
}

func StartTwo(n int, conc int) {
	// Start a single thread that subscribes to the redis stream
	metrics := metricsRequestStream(generateData(n)...)

	// Start multiple threads to process events coming off the redis stream
	sem := make(chan struct{}, conc)
	for m := range metrics {
		sem <- struct{}{}
		go func(b []byte) {
			defer func() {
				<-sem
			}()

			handleMetrics(b)
		}(m)
	}
}

func StartOne(n int, conc int) {
	wg := &sync.WaitGroup{}
	wg.Add(conc)

	// Start a single thread that subscribes to the redis stream
	metrics := metricsRequestStream(generateData(n)...)

	// Start multiple threads to process events coming off the redis stream
	for i := 0; i < conc; i++ {
		go func() {
			defer wg.Done()
		}()
		handleMetricsReadFromChan(metrics)
	}

	wg.Wait()
}

// Benchmarking the handle metrics function reading from a channel vs just taking a raw bytes
func Benchmark_HandleMetrics(b *testing.B) {
	benchmarks := []struct {
		name          string
		metricsStream <-chan []byte
		fn1           func(<-chan []byte)
		fn2           func([]byte)
	}{
		{
			name:          "handleMetrics- 1000 metrics",
			metricsStream: metricsRequestStream(generateData(10000)...),
			fn2:           handleMetrics,
		},
		{
			name:          "handleMetricsFromChan - 1000 metrics",
			metricsStream: metricsRequestStream(generateData(10000)...),
			fn1:           handleMetricsReadFromChan,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {

				if bm.fn2 != nil {
					for m := range bm.metricsStream {
						bm.fn2(m)
					}
				} else if bm.fn1 != nil {
					bm.fn1(bm.metricsStream)
				}

			}
		})
	}

}

func BenchmarkName(b *testing.B) {
	benchmarks := []struct {
		name          string
		metricsStream <-chan []byte
		fn            func(<-chan []byte)
	}{
		{
			name:          "single thread - 1000 metrics",
			metricsStream: metricsRequestStream(generateData(10000)...),
			fn:            singleThread,
		},
		{
			name:          "fanout - 1000 metrics",
			metricsStream: metricsRequestStream(generateData(10000)...),
			fn:            fanout,
		},
		{
			name:          "semaphore - 1000 metrics",
			metricsStream: metricsRequestStream(generateData(10000)...),
			fn:            semaphore,
		},
		{
			name:          "foo - 1000 metrics",
			metricsStream: metricsRequestStream(generateData(10000)...),
			fn:            foo,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				bm.fn(bm.metricsStream)
			}
		})
	}
}

func fanout(metrics <-chan []byte) {
	wg := &sync.WaitGroup{}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for m := range metrics {
				handleMetrics(m)
			}
		}()
	}

	wg.Wait()
}

func foo(metrics <-chan []byte) {
	wg := &sync.WaitGroup{}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			handleMetricsReadFromChan(metrics)
		}()
	}
}

func semaphore(metrics <-chan []byte) {
	sem := make(chan struct{}, 5)

	for m := range metrics {
		sem <- struct{}{}
		go func() {
			defer func() {
				<-sem
			}()
			handleMetrics(m)
		}()
	}
}

func singleThread(metrics <-chan []byte) {
	for m := range metrics {
		handleMetrics(m)
	}
}

func handleMetrics(b []byte) {
	mr := domain.MetricsRequest{}

	if err := jsoniter.Unmarshal(b, &mr); err != nil {
		panic(err)
	}
}

func handleMetricsReadFromChan(b <-chan []byte) {
	for {
		select {
		case v, ok := <-b:
			if !ok {
				return
			}

			mr := domain.MetricsRequest{}
			if err := jsoniter.Unmarshal(v, &mr); err != nil {
				panic(err)
			}
		}
	}
}

func metricsRequestStream(metricsRequests ...domain.MetricsRequest) <-chan []byte {
	out := make(chan []byte)

	go func() {
		defer close(out)

		for _, mr := range metricsRequests {
			b, err := jsoniter.Marshal(mr)
			if err != nil {
				panic(err)
			}

			out <- b
		}
	}()

	return out
}

func generateData(size int) []domain.MetricsRequest {
	data := make([]domain.MetricsRequest, 0)

	for i := 0; i < size; i++ {
		envID := 1
		if i%2 == 0 {
			envID = 2
		}

		data = append(data, domain.MetricsRequest{
			EnvironmentID: fmt.Sprintf("env-%d", envID),
			Metrics: clientgen.Metrics{
				MetricsData: &[]clientgen.MetricsData{
					{
						Attributes: []clientgen.KeyValue{
							{
								Key:   "FOOOOOOOOOOOOOOOOOOOOOOOOOOOOO",
								Value: "BAARRRRRRRRRRRRRRRRRRRRRRRRR",
							},
						},
						Count:       2,
						MetricsType: "Server",
						Timestamp:   time.Now().UnixMilli(),
					},
					{
						Attributes: []clientgen.KeyValue{
							{
								Key:   "FOOOOOOOOOOOOOOOOOOOOOOOOOOOOO",
								Value: "BAARRRRRRRRRRRRRRRRRRRRRRRRR",
							},
						},
						Count:       2,
						MetricsType: "Server",
						Timestamp:   time.Now().UnixMilli(),
					},
					{
						Attributes: []clientgen.KeyValue{
							{
								Key:   "FOOOOOOOOOOOOOOOOOOOOOOOOOOOOO",
								Value: "BAARRRRRRRRRRRRRRRRRRRRRRRRR",
							},
						},
						Count:       2,
						MetricsType: "Server",
						Timestamp:   time.Now().UnixMilli(),
					},
				},
				TargetData: &[]clientgen.TargetData{
					{
						Attributes: []clientgen.KeyValue{
							{
								Key:   "FOOOOOOOOOOOOOOOOOOOOOOOOOOOOO",
								Value: "BAARRRRRRRRRRRRRRRRRRRRRRRRR",
							},
						},
						Identifier: "HELOOOOOOOOO",
						Name:       "WORLDDDDDDDDDDD",
					},
					{
						Attributes: []clientgen.KeyValue{
							{
								Key:   "FOOOOOOOOOOOOOOOOOOOOOOOOOOOOO",
								Value: "BAARRRRRRRRRRRRRRRRRRRRRRRRR",
							},
						},
						Identifier: "HELOOOOOOOOO",
						Name:       "WORLDDDDDDDDDDD",
					},
					{
						Attributes: []clientgen.KeyValue{
							{
								Key:   "FOOOOOOOOOOOOOOOOOOOOOOOOOOOOO",
								Value: "BAARRRRRRRRRRRRRRRRRRRRRRRRR",
							},
						},
						Identifier: "HELOOOOOOOOO",
						Name:       "WORLDDDDDDDDDDD",
					},
				},
			},
		})
	}
	return data
}
