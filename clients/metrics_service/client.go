package metricsservice

import (
	"context"
	"fmt"
	"net/http"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/harness/ff-proxy/v2/log"
	"github.com/prometheus/client_golang/prometheus"
)

type key string

const (
	tokenKey key = "token"
)

// doer is a simple http client that gets passed to the generated admin client
// and injects the service token into the header before any requests are made
type doer struct {
	c     *http.Client
	token string
}

// Do injects the api-key header into the request
func (d doer) Do(r *http.Request) (*http.Response, error) {
	r.Header.Add("x-api-key", d.token)
	return d.c.Do(r)
}

type counter interface {
	prometheus.Collector
	WithLabelValues(lvs ...string) prometheus.Counter
}

// Client is a type for interacting with the Feature Flag Metric Service
type Client struct {
	log    log.Logger
	client clientgen.ClientWithResponsesInterface
	token  func() string

	sdkUsage         counter
	metricsForwarded counter
}

// NewClient creates a MetricStore
func NewClient(l log.Logger, addr string, token func() string, reg *prometheus.Registry) (Client, error) {
	l = l.With("component", "MetricServiceClient")
	client, err := clientgen.NewClientWithResponses(
		addr,
		clientgen.WithHTTPClient(doer{c: http.DefaultClient}),
	)
	if err != nil {
		return Client{}, err
	}

	m := Client{
		log:    l,
		client: client,
		token:  token,

		sdkUsage: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ff_proxy_sdk_usage",
				Help: "Tracks what SDKs are using the FF Proxy",
			},
			[]string{"envID", "sdk_type", "sdk_version", "sdk_language"},
		),
		metricsForwarded: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ff_proxy_metrics_forwarded",
				Help: "Tracks the number of metrics forwarded from the Proxy to SaaS Feature Flags",
			},
			[]string{"envID", "error"},
		),
	}

	reg.MustRegister(m.sdkUsage, m.metricsForwarded)
	return m, nil
}

func (c Client) PostMetrics(ctx context.Context, envID string, metric domain.MetricsRequest, clusterIdentifier string) (err error) {
	defer func() {
		errLabel := "false"
		if err != nil {
			errLabel = "true"
		}
		c.metricsForwarded.WithLabelValues(envID, errLabel).Inc()

		c.trackSDKUsage(metric)
	}()

	ctx = context.WithValue(ctx, tokenKey, c.token())
	res, err := c.client.PostMetricsWithResponse(ctx, envID, &clientgen.PostMetricsParams{Cluster: &clusterIdentifier}, clientgen.PostMetricsJSONRequestBody{
		MetricsData: metric.MetricsData,
		TargetData:  metric.TargetData,
	}, addAuthToken)
	if err != nil {
		return err
	}

	if res != nil && res.StatusCode() != 200 {
		return fmt.Errorf("got non 200 status code from feature flags: status_code=%d", res.StatusCode())
	}

	return nil
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

func addAuthToken(ctx context.Context, req *http.Request) error {
	token := ctx.Value(tokenKey)
	if token == nil || token == "" {
		return fmt.Errorf("no auth token exists in context")
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	return nil
}

func (c Client) trackSDKUsage(req domain.MetricsRequest) {
	if req.MetricsData == nil {
		return
	}

	for _, me := range *req.MetricsData {
		attrMap := createAttributeMap(me.Attributes)

		sdkType := getSDKType(attrMap)
		sdkVersion := getSDKVersion(attrMap)
		sdkLanguage := getSDKLanguage(attrMap)

		c.sdkUsage.WithLabelValues(req.EnvironmentID, sdkType, sdkVersion, sdkLanguage).Inc()
	}
}

// GetSDKLanguage returns the language or an empty string if its not found
func getSDKLanguage(m map[string]string) string {
	return m["SDK_LANGUAGE"]
}
