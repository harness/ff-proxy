package metricsservice

import (
	"fmt"
	"sync"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
)

// safeMetricsRequestMap is a map that aggregates metrics by
// environment, flag identifier & flag variation
type safeMetricsRequestMap struct {
	*sync.RWMutex
	detailed map[string]map[string]clientgen.MetricsData

	currentSize int
}

func newSafeMetricsRequestMap() *safeMetricsRequestMap {
	return &safeMetricsRequestMap{
		RWMutex:  &sync.RWMutex{},
		detailed: make(map[string]map[string]clientgen.MetricsData),
	}
}

func (s *safeMetricsRequestMap) size() int {
	s.RLock()
	defer s.RUnlock()
	return s.currentSize
}

func (s *safeMetricsRequestMap) flush() {
	s.Lock()
	defer s.Unlock()

	s.detailed = map[string]map[string]clientgen.MetricsData{}
	s.currentSize = 0
}

func (s *safeMetricsRequestMap) add(value domain.MetricsRequest) {
	s.Lock()
	defer s.Unlock()

	current, ok := s.detailed[value.EnvironmentID]
	if !ok {
		// If we don't already have metrics for this environment
		// cached then we can add them and be done
		m := map[string]clientgen.MetricsData{}

		for _, md := range domain.SafePtrDereference(value.MetricsData) {
			key := makeKey(value.EnvironmentID, md.Attributes)

			// Guard against getting duplicate flag variations
			// in the same payload.
			if v, ok2 := m[key]; ok2 {
				v.Count += md.Count
				m[key] = v
				continue
			}
			m[key] = md
		}

		s.detailed[value.EnvironmentID] = m

		// Because we're adding a new object to the map we'll
		// want to also append its size to the current map size.
		s.currentSize += value.Size
		return
	}

	// Otherwise we'll want to pull out the metrics for the current env
	// and increment counts for any flags we've already seen.
	for _, md := range domain.SafePtrDereference(value.MetricsData) {
		key := makeKey(value.EnvironmentID, md.Attributes)

		// If we've seen this key before then we've already got an
		// evaluation for this flag so we can just increment the count
		if v, ok := current[key]; ok {
			v.Count += md.Count
			current[key] = v
			continue
		}

		// If we haven't seen the key before then this is a new
		// evaluation for this flag so we just need to add it to our map
		// and increase the size
		current[key] = md

		s.currentSize += value.Size
	}
}

func (s *safeMetricsRequestMap) get() map[string]domain.MetricsRequest {
	// Take a copy an unlock so we don't block other thread
	// marshaling to the response type
	s.RLock()
	cpy := s.detailed
	s.RUnlock()

	result := map[string]domain.MetricsRequest{}

	for envID, detailedMap := range cpy {

		slice := []clientgen.MetricsData{}
		for _, v := range detailedMap {
			slice = append(slice, v)
		}

		result[envID] = domain.MetricsRequest{
			EnvironmentID: envID,
			Metrics: clientgen.Metrics{
				MetricsData: domain.ToPtr(slice),
			},
		}
	}

	return result
}

func makeKey(envID string, attributes []clientgen.KeyValue) string {
	var (
		variationIdentifier string
		flagIdentifier      string
		flagName            string
		sdkLanguage         string
		sdkVersion          string

		gotVariation   bool
		gotFlag        bool
		gotSDKLanguage bool
		gotSDKVersion  bool
	)

	// We need to get the flag and variation from the attributes to build up the key
	for _, attr := range attributes {
		if gotVariation && gotFlag && gotSDKLanguage && gotSDKVersion {
			break
		}

		if attr.Key == "variationIdentifier" {
			variationIdentifier = attr.Value
			gotVariation = true
			continue
		}

		if attr.Key == "featureIdentifier" {
			flagIdentifier = attr.Value
			gotFlag = true
			continue
		}

		if attr.Key == "SDK_LANGUAGE" {
			sdkLanguage = attr.Value
			gotSDKLanguage = true
			continue
		}

		if attr.Key == "SDK_VERSION" {
			sdkVersion = attr.Value
			gotSDKVersion = true
			continue
		}

		// If the flagIdentifier is already populated we don't need to bother
		// fetching the flagName
		if attr.Key == "featureName" && flagIdentifier == "" {
			flagName = attr.Value
			continue
		}
	}

	// Some SDKs send us a featureIdentifier attribute in the metrics but some
	// only send a featureName (but use the identifier as the value). So if we
	// don't find a featureIdentifier in the attributes we should use the name instead
	flagIdent := flagIdentifier
	if flagIdent == "" {
		flagIdent = flagName
	}

	return fmt.Sprintf("%s-%s-%s-%s-%s", envID, flagIdent, variationIdentifier, sdkLanguage, sdkVersion)
}
