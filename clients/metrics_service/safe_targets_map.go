package metricsservice

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

const (
	genericProxyTargetIdentifier = "__global__cf_target"
)

// safeTargetsMap is a type that stores targets from metrics requests
// and aggregates them by environment
type safeTargetsMap struct {
	*sync.RWMutex
	metrics     map[string]domain.MetricsRequest
	currentSize int
}

func newSafeTargetsMap() *safeTargetsMap {
	return &safeTargetsMap{
		RWMutex: &sync.RWMutex{},
		metrics: make(map[string]domain.MetricsRequest),
	}
}

func (m *safeTargetsMap) add(r domain.MetricsRequest) {
	m.Lock()
	defer m.Unlock()

	m.currentSize += r.Size

	// Store metrics to send later
	currentMetrics, ok := m.metrics[r.EnvironmentID]
	if !ok {
		m.metrics[r.EnvironmentID] = r
		return
	}

	incrSize := false

	if r.MetricsData != nil {
		incrSize = true

		if currentMetrics.MetricsData == nil {
			currentMetrics.MetricsData = &[]clientgen.MetricsData{}
		}
		newMetrics := append(*currentMetrics.MetricsData, *r.MetricsData...)
		currentMetrics.MetricsData = &newMetrics
	}

	if r.TargetData != nil {
		incrSize = true

		if currentMetrics.TargetData == nil {
			currentMetrics.TargetData = &[]clientgen.TargetData{}
		}

		newTargets := append(*currentMetrics.TargetData, *r.TargetData...)
		currentMetrics.TargetData = &newTargets
	}

	// As well as aggregating the metrics & target data we need to
	// 'merge' the size of the current aggregated object and the new one
	if incrSize {
		currentMetrics.Size += r.Size
	}

	m.metrics[r.EnvironmentID] = currentMetrics
}

// aggregate will convert and aggregate all the entries into the Metrics data and update new object.
func (m *safeTargetsMap) aggregate(r domain.MetricsRequest) ([]clientgen.MetricsData, error) {

	aggregatedMetricsMap := map[string]*clientgen.MetricsData{}
	// dereference here
	if r.MetricsData == nil {
		return []clientgen.MetricsData{}, errors.New("metrics data is nil")
	}

	metricsData := *r.MetricsData
	for i := 0; i < len(metricsData); i++ {
		keyName := getKeyEntry(r.EnvironmentID, &metricsData[i])
		//if we have a key we want to increment
		if _, ok := aggregatedMetricsMap[keyName]; ok {
			aggregatedMetricsMap[keyName].Count++
		} else {
			// update timestamp + create new map entry.
			metricsData[i].Timestamp = time.Now().UnixMilli()
			aggregatedMetricsMap[keyName] = &metricsData[i]
		}
	}
	// convert map of aggregatedMetrics to the array and then return it.
	aggregatedMetricsData := make([]clientgen.MetricsData, 0, len(aggregatedMetricsMap))
	for _, v := range aggregatedMetricsMap {
		aggregatedMetricsData = append(aggregatedMetricsData, *v)
	}
	// assign new list.
	return aggregatedMetricsData, nil
}

// getKeyEntry works out key entry for data item and updates the user.
func getKeyEntry(envID string, m *clientgen.MetricsData) string {
	var featureIdentifier, variationIdentifier, sdkName, sdkLanguage, sdkType, sdkVersion string
	// TODO is each of these items guaranteed ?
	// loop through the list of attributes for each data item
	for i := 0; i < len(m.Attributes); i++ {
		switch m.Attributes[i].Key {
		case "featureIdentifier":
			featureIdentifier = m.Attributes[i].Value
		case "variationIdentifier":
			variationIdentifier = m.Attributes[i].Value
		case "SDK_NAME":
			sdkName = m.Attributes[i].Value
		case "SDK_LANGUAGE":
			sdkLanguage = m.Attributes[i].Value
		case "SDK_TYPE":
			sdkType = m.Attributes[i].Value
		case "SDK_VERSION":
			sdkVersion = m.Attributes[i].Value
		case "target":
			// update the user to generic one
			// TODO what is the generic user?
			m.Attributes[i].Value = genericProxyTargetIdentifier
		}
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s-%s-%s", envID, featureIdentifier, variationIdentifier, sdkName, sdkLanguage, sdkType, sdkVersion)
}

func (m *safeTargetsMap) get() map[string]domain.MetricsRequest {
	m.RLock()
	defer m.RUnlock()

	return m.metrics
}

func (m *safeTargetsMap) flush() {
	m.Lock()
	defer m.Unlock()

	m.metrics = map[string]domain.MetricsRequest{}
	m.currentSize = 0
}

func (m *safeTargetsMap) size() int {
	m.RLock()
	defer m.RUnlock()

	return m.currentSize
}
