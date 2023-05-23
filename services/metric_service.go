package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/harness/ff-proxy/domain"
	clientgen "github.com/harness/ff-proxy/gen/client"
	"github.com/harness/ff-proxy/log"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	tokenKey = "token"
)

type counter interface {
	prometheus.Collector
	WithLabelValues(lvs ...string) prometheus.Counter
}

// MetricService is a type for interacting with the Feature Flag Metric Service
type MetricService struct {
	log         log.Logger
	accountID   string
	enabled     bool
	client      clientgen.ClientWithResponsesInterface
	metrics     map[string]domain.MetricsRequest
	tokens      map[string]string
	metricsLock *sync.Mutex

	sdkUsage counter
}

// NewMetricService creates a MetricService
func NewMetricService(l log.Logger, addr string, accountID string, tokens map[string]string, enabled bool, reg *prometheus.Registry) (MetricService, error) {
	l = l.With("component", "MetricServiceClient")
	client, err := clientgen.NewClientWithResponses(
		addr,
		clientgen.WithHTTPClient(doer{c: http.DefaultClient}),
	)
	if err != nil {
		return MetricService{}, err
	}

	m := MetricService{
		log:         l,
		accountID:   accountID,
		client:      client,
		enabled:     enabled,
		metrics:     map[string]domain.MetricsRequest{},
		tokens:      tokens,
		metricsLock: &sync.Mutex{},

		sdkUsage: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ff_proxy_sdk_usage",
				Help: "Tracks what SDKs are using the FF Proxy",
			},
			[]string{"envID", "sdk_type", "sdk_version", "sdk_language"}),
	}

	reg.MustRegister(m.sdkUsage)
	return m, nil
}

// StoreMetrics aggregates and stores metrics
func (m MetricService) StoreMetrics(ctx context.Context, req domain.MetricsRequest) error {
	if !m.enabled {
		return nil
	}

	m.metricsLock.Lock()
	defer func() {
		m.trackSDKUsage(req)
		m.metricsLock.Unlock()
	}()

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
			b, _ := io.ReadAll(res.HTTPResponse.Body)
			m.log.Info("failed metrics request", "request_url", res.HTTPResponse.Request.URL.String(), "body", string(b), "token", token)
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

func (m MetricService) trackSDKUsage(req domain.MetricsRequest) {
	if req.MetricsData == nil {
		return
	}

	for _, me := range *req.MetricsData {
		attrMap := createAttributeMap(me.Attributes)

		sdkType := getSDKType(attrMap)
		sdkVersion := getSDKVersion(attrMap)
		sdkLanguage := getSDKLanguage(attrMap)

		m.sdkUsage.WithLabelValues(req.EnvironmentID, sdkType, sdkVersion, sdkLanguage).Inc()
	}
}

func createAttributeMap(data []clientgen.KeyValue) map[string]string {
	result := map[string]string{}
	for _, kv := range data {
		result[kv.Key] = kv.Value
	}
	return result
}

// GetSDKType returns the sdk type or an empty string if its not found
func getSDKType(m map[string]string) string {
	return m["SDK_TYPE"]
}

// GetSDKVersion returns the version or an empty string if its not found
func getSDKVersion(m map[string]string) string {
	v, ok := m["SDK_VERSION"]
	if ok {
		return v
	}

	// TODO this should be SDK_VERSION - need to update java SDK
	v2, ok := m["JAR_VERSION"]
	if ok {
		return v2
	}

	return ""
}

// GetSDKLanguage returns the language or an empty string if its not found
func getSDKLanguage(m map[string]string) string {
	return m["SDK_LANGUAGE"]
}
