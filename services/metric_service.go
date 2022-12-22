package services

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	"github.com/harness/ff-proxy/log"
)

const (
	tokenKey = "token"
)

// MetricService is a type for interacting with the Feature Flag Metric Service
type MetricService struct {
	log         log.Logger
	accountID   string
	enabled     bool
	client      clientgen.ClientWithResponsesInterface
	metrics     map[string]domain.MetricsRequest
	tokens      map[string]string
	metricsLock sync.Mutex
}

// NewMetricService creates a MetricService
func NewMetricService(l log.Logger, addr string, accountID string, tokens map[string]string, enabled bool) (MetricService, error) {
	l = l.With("component", "MetricServiceClient")
	client, err := clientgen.NewClientWithResponses(
		addr,
		clientgen.WithHTTPClient(doer{c: http.DefaultClient}),
	)
	if err != nil {
		return MetricService{}, err
	}

	return MetricService{log: l, accountID: accountID, client: client, enabled: enabled, metrics: map[string]domain.MetricsRequest{}, tokens: tokens, metricsLock: sync.Mutex{}}, nil
}

// StoreMetrics aggregates and stores metrics
func (m MetricService) StoreMetrics(ctx context.Context, req domain.MetricsRequest) error {
	if !m.enabled {
		return nil
	}
	m.metricsLock.Lock()
	defer m.metricsLock.Unlock()
	// store metrics to send later
	if _, ok := m.metrics[req.EnvironmentID]; ok {
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

	for envID, metric := range metricsCopy {
		token, ok := m.tokens[envID]
		if !ok {
			m.log.Warn("No token found for environment. Skipping sending metrics for env.", "environment", envID)
			continue
		}
		ctx = context.WithValue(ctx, tokenKey, token)
		res, err := m.client.PostMetricsWithResponse(ctx, envID, &clientgen.PostMetricsParams{Cluster: &clusterIdentifier}, clientgen.PostMetricsJSONRequestBody{
			MetricsData: metric.MetricsData,
			TargetData:  metric.TargetData,
		}, addAuthToken)
		if err != nil {
			m.log.Error("sending metrics failed", "error", err)
		}
		if res != nil && res.StatusCode() != 200 {
			m.log.Error("sending metrics failed", "environment", envID, "status code", res.StatusCode())
		}
	}
}

func addAuthToken(ctx context.Context, req *http.Request) error {
	token := ctx.Value(tokenKey)
	if token == nil || token == "" {
		return fmt.Errorf("no auth token exists in context")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	return nil
}
