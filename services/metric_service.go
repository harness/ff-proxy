package services

import (
	"context"
	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	"github.com/harness/ff-proxy/log"
	"net/http"
	"sync"
)

// MetricService is a type for interacting with the Feature Flag Metric Service
type MetricService struct {
	log    log.Logger
	accountID string
	enabled bool
	client clientgen.ClientWithResponsesInterface
	metrics map[string]domain.MetricsRequest
	metricsLock sync.Mutex
}

// NewMetricService creates a MetricService
func NewMetricService(l log.Logger, addr string, accountID string, serviceToken string, enabled bool) (MetricService, error) {
	l = l.With("component", "MetricServiceClient")
	client, err := clientgen.NewClientWithResponses(
		addr,
		clientgen.WithHTTPClient(doer{c: http.DefaultClient, token: serviceToken}),
		)
	if err != nil {
		return MetricService{}, err
	}

	return MetricService{log: l, accountID: accountID, client: client, enabled: enabled, metrics: map[string]domain.MetricsRequest{}, metricsLock: sync.Mutex{}}, nil
}

// StoreMetrics aggregates and stores metrics
func (m MetricService) StoreMetrics(ctx context.Context, req domain.MetricsRequest) error {
	if !m.enabled {
		return nil
	}
	m.metricsLock.Lock()
	defer m.metricsLock.Unlock()
	// store metrics to send later
	if _,ok := m.metrics[req.EnvironmentID]; ok {
		currentMetrics := m.metrics[req.EnvironmentID]
		if req.MetricsData != nil {
			newMetrics := append(*currentMetrics.MetricsData, *req.MetricsData...)
			currentMetrics.MetricsData = &newMetrics
		}
		if req.TargetData != nil {
			newTargets := append(*currentMetrics.TargetData, *req.TargetData...)
			currentMetrics.TargetData = &newTargets
		}

		m.metrics[req.EnvironmentID] = currentMetrics
	} else {
		m.metrics[req.EnvironmentID] = req
	}

	return nil
}

// SendMetrics forwards stored metrics to the SaaS platform
func (m MetricService) SendMetrics(ctx context.Context, clusterIdentifier string) {
	// copy metrics before sending so we don't hog the lock for network requests
	m.metricsLock.Lock()
	metricsCopy := map[string]domain.MetricsRequest{}
	for key, val := range m.metrics {
		metricsCopy[key] = val
		delete(m.metrics, key)
	}
	m.metrics = make(map[string]domain.MetricsRequest)
	m.metricsLock.Unlock()
	clusterParam := clientgen.ClusterQueryOptionalParam(clusterIdentifier)

	for envID, metric := range metricsCopy {

		res, err := m.client.PostMetricsWithResponse(ctx, clientgen.EnvironmentPathParam(envID), &clientgen.PostMetricsParams{Cluster: &clusterParam}, clientgen.PostMetricsJSONRequestBody{
			MetricsData: metric.MetricsData,
			TargetData:  metric.TargetData,
		}, m.addAccountQueryParam)
		if err != nil {
			m.log.Error("sending metrics failed", "error", err)
		}
		if res != nil && res.StatusCode() != 200 {
			m.log.Error("sending metrics failed", "environment", envID, "status code", res.StatusCode())
		}
	}
}

func (m MetricService) addAccountQueryParam (ctx context.Context, req *http.Request) error {
	queryParams := req.URL.Query()
	queryParams.Add("accountIdentifier", m.accountID)
	req.URL.RawQuery = queryParams.Encode()
	return nil
}
