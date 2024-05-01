package metricsservice

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

const (
	testDuration = 1 * time.Second
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
		metricsMap *safeTargetsMap
		args       args
		expected   expected
	}{
		"Given I add one element to an empty map": {
			metricsMap: newSafeTargetsMap(),
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
			metricsMap: &safeTargetsMap{
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
			metricsMap: &safeTargetsMap{
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
		metricsMap *safeTargetsMap
		expected   expected
	}{
		"Given I have an empty metrics map": {
			metricsMap: newSafeTargetsMap(),
			expected:   expected{data: make(map[string]domain.MetricsRequest)},
		},
		"Given I have a metrics map with one item in it": {
			metricsMap: &safeTargetsMap{
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
		metricsMap *safeTargetsMap
		expected   expected
	}{
		"Given I have an empty metrics map": {
			metricsMap: newSafeTargetsMap(),
			expected:   expected{size: 0},
		},
		"Given I have a metrics map with a size of 11": {
			metricsMap: &safeTargetsMap{
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
		metricsMap *safeTargetsMap
		expected   expected
	}{
		"Given I have an empty metrics map and I call flush": {
			metricsMap: newSafeTargetsMap(),
			expected: expected{
				data: make(map[string]domain.MetricsRequest),
				size: 0,
			},
		},
		"Given I have a populated metrics map and I call flush": {
			metricsMap: &safeTargetsMap{
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

func TestMap_aggregate(t *testing.T) {

	mr := domain.MetricsRequest{
		Size:          6,
		EnvironmentID: "123",
		Metrics: clientgen.Metrics{
			MetricsData: &[]clientgen.MetricsData{
				{
					Attributes:  createAttributes("testTarget1", "featureID_1", "variationID_1", "JavaScript", "JavaScript", "client", "1.21.0"),
					Count:       1,
					MetricsType: "FFMETRICS",
					Timestamp:   111,
				},
				{
					Attributes:  createAttributes("testTarget2", "featureID_1", "variationID_1", "JavaScript", "JavaScript", "client", "1.21.0"),
					Count:       1,
					MetricsType: "FFMETRICS",
					Timestamp:   111,
				},
				{
					Attributes:  createAttributes("testTarget3", "featureID_1", "variationID_1", "JavaScript", "JavaScript", "client", "1.21.0"),
					Count:       1,
					MetricsType: "FFMETRICS",
					Timestamp:   111,
				},
				{
					Attributes:  createAttributes("testTarget4", "featureID_2", "variationID_1", "JavaScript", "JavaScript", "client", "1.21.0"),
					Count:       1,
					MetricsType: "FFMETRICS",
					Timestamp:   111,
				},
				{
					Attributes:  createAttributes("testTarget5", "featureID_1", "variationID_1", "JavaScript", "JavaScript", "client", "1.21.0"),
					Count:       1,
					MetricsType: "FFMETRICS",
					Timestamp:   111,
				},
				{
					Attributes:  createAttributes("testTarget6", "featureID_2", "variationID_1", "JavaScript", "JavaScript", "client", "1.21.0"),
					Count:       1,
					MetricsType: "FFMETRICS",
					Timestamp:   111,
				},
			},
		},
	}
	mrNil := domain.MetricsRequest{
		Size:          6,
		EnvironmentID: "123",
		Metrics: clientgen.Metrics{
			MetricsData: nil,
		},
	}

	type args struct {
		metricRequest domain.MetricsRequest
	}

	type expected struct {
		shouldError bool
		err         error
		data        []clientgen.MetricsData
		size        int
	}

	testCases := map[string]struct {
		metricsMap *safeTargetsMap
		args       args
		expected   expected
	}{
		"Given I have same flags evaluated by different targets - aggregate records": {
			metricsMap: &safeTargetsMap{
				RWMutex: &sync.RWMutex{},
				metrics: map[string]domain.MetricsRequest{
					"123": mr,
				},
				currentSize: mr.Size,
			},
			args: args{
				metricRequest: mr,
			},
			expected: expected{
				data: []clientgen.MetricsData{
					{
						Attributes:  createAttributes(genericProxyTargetIdentifier, "featureID_1", "variationID_1", "JavaScript", "JavaScript", "client", "1.21.0"),
						Count:       4,
						MetricsType: "FFMETRICS",
						Timestamp:   time.Now().UnixMilli(),
					},
					{
						Attributes:  createAttributes(genericProxyTargetIdentifier, "featureID_2", "variationID_1", "JavaScript", "JavaScript", "client", "1.21.0"),
						Count:       2,
						MetricsType: "FFMETRICS",
						Timestamp:   time.Now().UnixMilli(),
					},
				},
				size:        2,
				shouldError: false,
			},
		},

		"Given I nil metrics data should error": {
			metricsMap: &safeTargetsMap{
				RWMutex: &sync.RWMutex{},
				metrics: map[string]domain.MetricsRequest{
					"123": mr,
				},
				currentSize: mr.Size,
			},
			args: args{
				metricRequest: mrNil,
			},
			expected: expected{
				data:        []clientgen.MetricsData{},
				size:        0,
				shouldError: true,
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			aggregated, err := tc.metricsMap.aggregate(tc.args.metricRequest)
			if tc.expected.shouldError {
				assert.Error(t, err, fmt.Errorf("metrics data is nil"))
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tc.expected.size, len(aggregated))
			}

		})
	}
}

func createAttributes(target, featureIdentifier, variationIdentifier, sdkName, sdkLanguage, sdkType, sdkVersion string) []clientgen.KeyValue {
	return []clientgen.KeyValue{
		{
			Key:   "featureIdentifier",
			Value: featureIdentifier,
		},
		{
			Key:   "featureName",
			Value: featureIdentifier,
		},
		{
			Key:   "featureIdentifier",
			Value: featureIdentifier,
		},
		{
			Key:   "variationIdentifier",
			Value: variationIdentifier,
		},
		{
			Key:   "target",
			Value: target,
		},
		{
			Key:   "SDK_NAME",
			Value: sdkName,
		},
		{
			Key:   "SDK_LANGUAGE",
			Value: sdkLanguage,
		},
		{
			Key:   "SDK_TYPE",
			Value: sdkType,
		},
		{
			Key:   "SDK_VERSION",
			Value: sdkVersion,
		},
	}
}
