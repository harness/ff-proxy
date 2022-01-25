 package services

 import (
	 "context"
	 "fmt"
	 "github.com/harness/ff-proxy/log"
	 "net/http"
	 "net/url"
	 "testing"

	 "github.com/harness/ff-proxy/domain"
	 clientgen "github.com/harness/ff-proxy/gen/client"
	 "github.com/stretchr/testify/assert"
 )

func (m mockService) PostMetricsWithResponse(ctx context.Context, environment clientgen.EnvironmentPathParam, params *clientgen.PostMetricsParams, body clientgen.PostMetricsJSONRequestBody, reqEditors ...clientgen.RequestEditorFn) (*clientgen.PostMetricsResponse, error) {
	var err error
	if m.postMetricsWithResp != nil {
		err = m.postMetricsWithResp()
	}

	return &clientgen.PostMetricsResponse{}, err
}

var env123MetricsFlag1 = domain.MetricsRequest{
	EnvironmentID: "123",
	TargetData:    []domain.TargetData{
		{
			Identifier: "targetID",
			Name: "target name",
			Attributes: []domain.KeyValue{{Key: "targetkey", Value: "targetvalue"}},
		},
	},
	MetricsData:   []domain.MetricsData{
		{
			Timestamp: int64(1234),
			Count: 1,
			MetricsType: "FFMETRICS",
			Attributes: []domain.KeyValue{{Key: "featureIdentifier", Value: "flag1"}},
		},
	},
}

 var env123MetricsFlag2 = domain.MetricsRequest{
	 EnvironmentID: "123",
	 TargetData:    []domain.TargetData{},
	 MetricsData:   []domain.MetricsData{
		 {
			 Timestamp: int64(5678),
			 Count: 2,
			 MetricsType: "FFMETRICS",
			 Attributes: []domain.KeyValue{{Key: "featureIdentifier", Value: "flag2"}},
		 },
	 },
 }

 var env456MetricsFlag1 = domain.MetricsRequest{
	 EnvironmentID: "456",
	 TargetData:    []domain.TargetData{},
	 MetricsData:   []domain.MetricsData{
		 {
			 Timestamp: int64(2345),
			 Count: 1,
			 MetricsType: "FFMETRICS",
			 Attributes: []domain.KeyValue{{Key: "featureIdentifier", Value: "flag1"}},
		 },
	 },
 }

func TestMetricService_StoreMetrics(t *testing.T) {
	testCases := map[string]struct {
		metrics     []domain.MetricsRequest
		expected    map[string]domain.MetricsRequest
	}{
		"Given I save one environments metrics": {
			metrics:  []domain.MetricsRequest{env123MetricsFlag1},
			expected: map[string]domain.MetricsRequest{"123": env123MetricsFlag1},
		},
		"Given I save two sets of metrics for one environment we combine them": {
			metrics:  []domain.MetricsRequest{env123MetricsFlag1, env123MetricsFlag2},
			expected: map[string]domain.MetricsRequest{"123": {
				EnvironmentID: "123",
				TargetData:    []domain.TargetData{env123MetricsFlag1.TargetData[0]},
				MetricsData:   []domain.MetricsData{env123MetricsFlag1.MetricsData[0], env123MetricsFlag2.MetricsData[0]},
			}},
		},
		"Given I save two sets of metrics for different environments": {
			metrics:  []domain.MetricsRequest{env123MetricsFlag1, env123MetricsFlag2, env456MetricsFlag1},
			expected: map[string]domain.MetricsRequest{"123": {
				EnvironmentID: "123",
				TargetData:    []domain.TargetData{env123MetricsFlag1.TargetData[0]},
				MetricsData:   []domain.MetricsData{env123MetricsFlag1.MetricsData[0], env123MetricsFlag2.MetricsData[0]},
			},
			"456": env456MetricsFlag1},
		},
	}

	for desc, tc := range testCases {
		tc := tc
		t.Run(desc, func(t *testing.T) {

			metricService := MetricService{metrics: map[string]domain.MetricsRequest{}, enabled: true}

			for _, metric := range tc.metrics {
				metricService.StoreMetrics(context.Background(), metric)
			}

			actual := metricService.metrics

			assert.Equal(t, tc.expected, actual)

		})
	}
}

 func TestMetricService_SendMetrics(t *testing.T) {
	 testCases := map[string]struct {
		 metrics     map[string]domain.MetricsRequest
		 postMetricsWithResp func() error
	 }{
		 "Given I send one environments metrics": {
			 metrics: map[string]domain.MetricsRequest{"123": env123MetricsFlag1},
			 postMetricsWithResp: func() error {
				 return nil
			 },
		 },
		 "Given I have an error sending metrics": {
			 metrics: map[string]domain.MetricsRequest{"123": env123MetricsFlag1},
			 postMetricsWithResp: func() error {
				 return fmt.Errorf("stuff went wrong")
			 },
		 },
	 }

	 for desc, tc := range testCases {
		 tc := tc
		 t.Run(desc, func(t *testing.T) {
		 	logger, _ := log.NewStructuredLogger(true)
			 metricService := MetricService{metrics: tc.metrics, client: mockService{postMetricsWithResp: tc.postMetricsWithResp}, log: logger}

			 metricService.SendMetrics(context.Background())

			 // check metrics are cleared after sending
			 actual := metricService.metrics
			 assert.Equal(t, map[string]domain.MetricsRequest{}, actual)

		 })
	 }
 }

 func TestMetricService_addAccountQueryParam(t *testing.T) {
	 testCases := map[string]struct {
		 baseURL string
		 accountIdentifier string
		 expectedURL string
	 }{
		 "Given I have no query params": {
			baseURL: "localhost:8000/env/123/metrics",
			accountIdentifier: "account1",
			expectedURL: "localhost:8000/env/123/metrics?accountIdentifier=account1",
		 },
		 "Given I have existing query params": {
			 baseURL: "localhost:8000/env/123/metrics?firstParam=test",
			 accountIdentifier: "account1",
			 expectedURL: "localhost:8000/env/123/metrics?accountIdentifier=account1&firstParam=test",
		 },
	 }

	 for desc, tc := range testCases {
		 tc := tc
		 t.Run(desc, func(t *testing.T) {
			 logger, _ := log.NewStructuredLogger(true)
			 // create metric service
			 metricService, _ := NewMetricService(logger, tc.baseURL, tc.accountIdentifier, "token", true)

			 startURL, _ := url.Parse(tc.baseURL)
			 req := http.Request{URL: startURL}
			 metricService.addAccountQueryParam(context.Background(), &req)

			 assert.Equal(t, tc.expectedURL, req.URL.String())

		 })
	 }
 }

