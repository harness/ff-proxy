package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

type mockCache struct {
	set    func() error
	get    func() error
	delete func() error
	scan   func() (map[string]string, error)
}

func (m mockCache) Scan(ctx context.Context, key string) (map[string]string, error) {
	return m.scan()
}

func (m mockCache) Set(ctx context.Context, key string, value interface{}) error {
	return m.set()
}

func (m mockCache) Get(ctx context.Context, key string, value interface{}) error {
	return m.get()
}

func (m mockCache) Delete(ctx context.Context, key string) error {
	return m.delete()
}

func (m mockCache) Keys(ctx context.Context, key string) ([]string, error) {
	return []string{}, nil
}

func (m mockCache) HealthCheck(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

type mockCounter struct {
	prometheus.Collector
	counts int
	labels []string
}

func (m *mockCounter) WithLabelValues(lvs ...string) prometheus.Counter {
	m.counts++
	m.labels = append(m.labels, lvs...)
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name: "",
	})
}

type mockObserver struct {
	observations int
}

func (m *mockObserver) Observe(v float64) {
	m.observations++
}

type mockHistogram struct {
	prometheus.Collector
	observer *mockObserver
	labels   []string
}

func (m *mockHistogram) WithLabelValues(lvs ...string) prometheus.Observer {
	return m.observer
}

func TestCacheMetrics_Set(t *testing.T) {
	type args struct {
		key   string
		value string

		cache mockCache
	}

	type result struct {
		observations int
		labels       []string
	}

	testCases := map[string]struct {
		args          args
		shouldErr     bool
		writeDuration *mockHistogram
		writeCount    *mockCounter
		expected      result
	}{
		"Given I call Set and the decorated cache errors": {
			args: args{
				key:   "foo",
				value: "foo",

				cache: mockCache{
					set: func() error {
						return errors.New("a set error")
					},
				},
			},

			shouldErr:     true,
			writeDuration: &mockHistogram{observer: &mockObserver{}},
			writeCount:    &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"true"},
			},
		},
		"Given I call Set and the decorated cache doesn't error": {
			args: args{
				key:   "foo",
				value: "foo",

				cache: mockCache{
					set: func() error {
						return nil
					},
				},
			},

			shouldErr:     false,
			writeDuration: &mockHistogram{observer: &mockObserver{}},
			writeCount:    &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"false"},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := MetricsCache{
				next:          tc.args.cache,
				writeDuration: tc.writeDuration,
				writeCount:    tc.writeCount,
			}
			err := c.Set(context.Background(), tc.args.key, tc.args.value)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			t.Log("Then the writeDuration should be observed once")
			assert.Equal(t, tc.expected.observations, tc.writeDuration.observer.observations)

			t.Log("And the writeCount should be observed once")
			assert.Equal(t, tc.expected.observations, tc.writeCount.counts)

			t.Logf("And the writeCount metric should have the labels: %v", tc.expected.labels)
			assert.Equal(t, tc.expected.labels, tc.writeCount.labels)

		})
	}
}

func TestCacheMetrics_Get(t *testing.T) {
	type args struct {
		key   string
		value string

		cache mockCache
	}

	type result struct {
		observations int
		labels       []string
	}

	testCases := map[string]struct {
		args         args
		shouldErr    bool
		readDuration *mockHistogram
		readCount    *mockCounter
		expected     result
	}{
		"Given I call Get and the decorated cache errors": {
			args: args{
				key:   "foo",
				value: "foo",

				cache: mockCache{
					get: func() error { return errors.New("a get error") },
				},
			},

			shouldErr:    true,
			readDuration: &mockHistogram{observer: &mockObserver{}},
			readCount:    &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"true"},
			},
		},
		"Given I call Get and the decorated cache doesn't error": {
			args: args{
				key:   "foo",
				value: "foo",

				cache: mockCache{
					get: func() error { return nil },
				},
			},

			shouldErr:    false,
			readDuration: &mockHistogram{observer: &mockObserver{}},
			readCount:    &mockCounter{},
			expected: result{
				observations: 1,
				labels:       []string{"false"},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := MetricsCache{
				next:         tc.args.cache,
				readDuration: tc.readDuration,
				readCount:    tc.readCount,
			}
			err := c.Get(context.Background(), tc.args.key, tc.args.value)
			if tc.shouldErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}

			t.Log("Then the readDuration should be observed once")
			assert.Equal(t, tc.expected.observations, tc.readDuration.observer.observations)

			t.Log("Then the readCount should be observed once")
			assert.Equal(t, tc.expected.observations, tc.readCount.counts)

			t.Logf("And the readCount metric should have the labels: %v", tc.expected.labels)
			assert.Equal(t, tc.expected.labels, tc.readCount.labels)
		})
	}
}

func TestCacheMetrics_Delete(t *testing.T) {
	type args struct {
		key string

		cache mockCache
	}

	type result struct {
		observations int
		labels       []string
	}

	testCases := map[string]struct {
		args          args
		shouldErr     bool
		deleteDuraton *mockHistogram
		deleteCount   *mockCounter
		expected      result
	}{
		"Given I call Delete and the underlying cache errors": {
			args: args{
				key: "foo",

				cache: mockCache{
					delete: func() error { return errors.New("delete error ") },
				},
			},

			shouldErr:     true,
			deleteDuraton: &mockHistogram{observer: &mockObserver{}},
			deleteCount:   &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"true"},
			},
		},
		"Given I call Delete and the underlying cache doesn't erorr": {
			args: args{
				key: "foo",

				cache: mockCache{
					delete: func() error { return nil },
				},
			},

			shouldErr:     true,
			deleteDuraton: &mockHistogram{observer: &mockObserver{}},
			deleteCount:   &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"false"},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := MetricsCache{
				next:           tc.args.cache,
				deleteDuration: tc.deleteDuraton,
				deleteCount:    tc.deleteCount,
			}

			c.Delete(context.Background(), tc.args.key)

			t.Log("Then the deleteCount should be observed once")
			assert.Equal(t, tc.expected.observations, tc.deleteCount.counts)

			t.Log("Then the deleteDuration should be observed once")
			assert.Equal(t, tc.expected.observations, tc.deleteDuraton.observer.observations)

			t.Logf("And the deleteCount metric should have the labels: %v", tc.expected.labels)
			assert.Equal(t, tc.expected.labels, tc.deleteCount.labels)
		})
	}
}

func TestNewCacheMetrics(t *testing.T) {
	// Just test that we don't panic registering the metrics
	reg := prometheus.NewRegistry()
	_ = NewMetricsCache("hello", reg, nil)
}
