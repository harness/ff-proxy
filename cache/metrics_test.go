package cache

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

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

type mockValue struct {
	marshal   func() ([]byte, error)
	unmarshal func([]byte) error
}

func (m mockValue) UnmarshalBinary(data []byte) error {
	return m.unmarshal(data)
}

func (m mockValue) MarshalBinary() (data []byte, err error) {
	return m.marshal()
}

func TestCacheMetrics_Set(t *testing.T) {
	type args struct {
		key   string
		field string
		value mockValue
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
				field: "bar",
				value: mockValue{
					marshal: func() ([]byte, error) { return nil, errors.New("an error") },
				},
			},

			shouldErr:     true,
			writeDuration: &mockHistogram{observer: &mockObserver{}},
			writeCount:    &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"foo", "Set", "true"},
			},
		},
		"Given I call Set and the decorated cache doesn't error": {
			args: args{
				key:   "foo",
				field: "bar",
				value: mockValue{
					marshal: func() ([]byte, error) { return nil, nil },
				},
			},

			shouldErr:     false,
			writeDuration: &mockHistogram{observer: &mockObserver{}},
			writeCount:    &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"foo", "Set", "false"},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := MetricsCache{
				next:          NewMemCache(),
				writeDuration: tc.writeDuration,
				writeCount:    tc.writeCount,
			}
			err := c.Set(context.Background(), tc.args.key, tc.args.field, tc.args.value)
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
		field string
		value mockValue
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
				field: "bar",
				value: mockValue{
					unmarshal: func(b []byte) error { return errors.New("an error") },
				},
			},

			shouldErr:    true,
			readDuration: &mockHistogram{observer: &mockObserver{}},
			readCount:    &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"foo", "Get", "true"},
			},
		},
		"Given I call Get and the decorated cache doesn't error": {
			args: args{
				key:   "foo",
				field: "bar",
				value: mockValue{
					unmarshal: func(b []byte) error { return nil },
				},
			},

			shouldErr:    false,
			readDuration: &mockHistogram{observer: &mockObserver{}},
			readCount:    &mockCounter{},
			expected: result{
				observations: 1,
				labels:       []string{"foo", "Get", "false"},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := MetricsCache{
				next: MemCache{
					RWMutex: &sync.RWMutex{},
					data: map[string]map[string][]byte{
						"foo": map[string][]byte{"bar": []byte("hello world")},
					},
				},
				readDuration: tc.readDuration,
				readCount:    tc.readCount,
			}
			err := c.Get(context.Background(), tc.args.key, tc.args.field, tc.args.value)
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

func TestCacheMetrics_GetAll(t *testing.T) {
	type args struct {
		key       string
		cacheData map[string]map[string][]byte
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
		"Given I call GetAll and the decorated cache errors": {
			args: args{
				key:       "foo",
				cacheData: map[string]map[string][]byte{},
			},

			shouldErr:    true,
			readDuration: &mockHistogram{observer: &mockObserver{}},
			readCount:    &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"foo", "GetAll", "true"},
			},
		},
		"Given I call GetAll and the decorated cache doesn't error": {
			args: args{
				key: "foo",
				cacheData: map[string]map[string][]byte{
					"foo": map[string][]byte{"bar": []byte("hello world")},
				},
			},

			shouldErr:    false,
			readDuration: &mockHistogram{observer: &mockObserver{}},
			readCount:    &mockCounter{},
			expected: result{
				observations: 1,
				labels:       []string{"foo", "GetAll", "false"},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := MetricsCache{
				next: MemCache{
					RWMutex: &sync.RWMutex{},
					data:    tc.args.cacheData,
				},
				readDuration: tc.readDuration,
				readCount:    tc.readCount,
			}

			_, err := c.GetAll(context.Background(), tc.args.key)
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

func TestCacheMetrics_RemoveAll(t *testing.T) {
	type args struct {
		key string
	}

	type result struct {
		observations int
		labels       []string
	}

	testCases := map[string]struct {
		args           args
		shouldErr      bool
		deleteDuration *mockHistogram
		deleteCount    *mockCounter
		expected       result
	}{
		"Given I call RemoveAll": {
			args: args{
				key: "foo",
			},

			shouldErr:      true,
			deleteDuration: &mockHistogram{observer: &mockObserver{}},
			deleteCount:    &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"foo", "RemoveAll"},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := MetricsCache{
				next: MemCache{
					RWMutex: &sync.RWMutex{},
					data:    map[string]map[string][]byte{},
				},
				deleteDuration: tc.deleteDuration,
				deleteCount:    tc.deleteCount,
			}

			c.RemoveAll(context.Background(), tc.args.key)

			t.Log("Then the deleteDuration should be observed once")
			assert.Equal(t, tc.expected.observations, tc.deleteDuration.observer.observations)

			t.Log("Then the deleteCount should be observed once")
			assert.Equal(t, tc.expected.observations, tc.deleteCount.counts)

			t.Logf("And the deleteCount metric should have the labels: %v", tc.expected.labels)
			assert.Equal(t, tc.expected.labels, tc.deleteCount.labels)
		})
	}
}

func TestCacheMetrics_Remove(t *testing.T) {
	type args struct {
		key   string
		field string
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
		"Given I call Remove": {
			args: args{
				key:   "foo",
				field: "bar",
			},

			shouldErr:     true,
			deleteDuraton: &mockHistogram{observer: &mockObserver{}},
			deleteCount:   &mockCounter{},

			expected: result{
				observations: 1,
				labels:       []string{"foo", "Remove"},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			c := MetricsCache{
				next: MemCache{
					RWMutex: &sync.RWMutex{},
					data:    map[string]map[string][]byte{},
				},
				deleteDuration: tc.deleteDuraton,
				deleteCount:    tc.deleteCount,
			}

			c.Remove(context.Background(), tc.args.key, tc.args.field)

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
