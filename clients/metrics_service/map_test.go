package metricsservice

import (
	"sync"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/stretchr/testify/assert"
)

func TestMap_add(t *testing.T) {

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

	mr123Again := domain.MetricsRequest{
		Size:          2,
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
					Identifier: "Hello",
					Name:       "World",
				},
			},
		},
	}

	mr456 := domain.MetricsRequest{
		Size:          8,
		EnvironmentID: "456",
		Metrics:       clientgen.Metrics{},
	}

	type args struct {
		metricRequest domain.MetricsRequest
	}

	type expected struct {
		data map[string]domain.MetricsRequest
		size int
	}

	testCases := map[string]struct {
		metricsMap *metricsMap
		args       args
		expected   expected
	}{
		"Given I add one element to an empty map": {
			metricsMap: newMetricsMap(),
			args: args{
				metricRequest: mr123,
			},
			expected: expected{
				data: map[string]domain.MetricsRequest{
					"123": mr123,
				},
				size: 7,
			},
		},
		"Given I add a second element for a different environment": {
			metricsMap: &metricsMap{
				RWMutex: &sync.RWMutex{},
				metrics: map[string]domain.MetricsRequest{
					"123": mr123,
				},
				currentSize: mr123.Size,
			},
			args: args{
				metricRequest: mr456,
			},
			expected: expected{
				data: map[string]domain.MetricsRequest{
					"123": mr123,
					"456": mr456,
				},
				size: 15,
			},
		},
		"Given I add a second element for the same environment different environment": {
			metricsMap: &metricsMap{
				RWMutex: &sync.RWMutex{},
				metrics: map[string]domain.MetricsRequest{
					"123": mr123,
				},
				currentSize: mr123.Size,
			},
			args: args{
				metricRequest: mr123Again,
			},
			expected: expected{
				data: map[string]domain.MetricsRequest{
					"123": domain.MetricsRequest{
						Size:          9,
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: mergeSlices(*mr123.MetricsData, *mr123Again.MetricsData),
							TargetData:  mergeSlices(*mr123.TargetData, *mr123Again.TargetData),
						},
					},
				},
				size: 9,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			tc.metricsMap.add(tc.args.metricRequest)

			assert.Equal(t, tc.expected.data, tc.metricsMap.metrics)
			assert.Equal(t, tc.expected.size, tc.metricsMap.currentSize)
		})
	}
}

func mergeSlices[T any](s1, s2 []T) *[]T {
	merged := make([]T, 0, len(s1)+len(s2))
	merged = append(merged, s1...)
	merged = append(merged, s2...)
	return &merged
}

func TestMetricsMap_get(t *testing.T) {

	type expected struct {
		data map[string]domain.MetricsRequest
	}

	testCases := map[string]struct {
		metricsMap *metricsMap
		expected   expected
	}{
		"Given I have an empty metrics map": {
			metricsMap: newMetricsMap(),
			expected:   expected{data: make(map[string]domain.MetricsRequest)},
		},
		"Given I have a metrics map with one item in it": {
			metricsMap: &metricsMap{
				RWMutex: &sync.RWMutex{},
				metrics: map[string]domain.MetricsRequest{
					"123": domain.MetricsRequest{
						Size:          5,
						EnvironmentID: "123",
						Metrics:       clientgen.Metrics{},
					},
				},
				currentSize: 5,
			},
			expected: expected{
				data: map[string]domain.MetricsRequest{
					"123": domain.MetricsRequest{
						Size:          5,
						EnvironmentID: "123",
						Metrics:       clientgen.Metrics{},
					},
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			actual := tc.metricsMap.get()
			assert.Equal(t, tc.expected.data, actual)

		})
	}
}

func TestMetricsMap_size(t *testing.T) {

	type expected struct {
		size int
	}

	testCases := map[string]struct {
		metricsMap *metricsMap
		expected   expected
	}{
		"Given I have an empty metrics map": {
			metricsMap: newMetricsMap(),
			expected:   expected{size: 0},
		},
		"Given I have a metrics map with a size of 11": {
			metricsMap: &metricsMap{
				RWMutex:     &sync.RWMutex{},
				metrics:     nil,
				currentSize: 11,
			},
			expected: expected{size: 11},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			actual := tc.metricsMap.size()
			assert.Equal(t, tc.expected.size, actual)
		})
	}
}

func TestMetricsMap_flush(t *testing.T) {

	type expected struct {
		data map[string]domain.MetricsRequest
		size int
	}

	testCases := map[string]struct {
		metricsMap *metricsMap
		expected   expected
	}{
		"Given I have an empty metrics map and I call flush": {
			metricsMap: newMetricsMap(),
			expected: expected{
				data: make(map[string]domain.MetricsRequest),
				size: 0,
			},
		},
		"Given I have a populated metrics map and I call flush": {
			metricsMap: &metricsMap{
				RWMutex: &sync.RWMutex{},
				metrics: map[string]domain.MetricsRequest{
					"123": domain.MetricsRequest{},
				},
				currentSize: 3,
			},
			expected: expected{
				data: make(map[string]domain.MetricsRequest),
				size: 0,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {
			tc.metricsMap.flush()

			assert.Equal(t, tc.expected.data, tc.metricsMap.metrics)
			assert.Equal(t, tc.expected.size, tc.metricsMap.currentSize)
		})
	}
}
