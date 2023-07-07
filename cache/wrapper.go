package cache

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/harness/ff-golang-server-sdk/rest"

	"github.com/harness/ff-golang-server-sdk/dto"
	"github.com/harness/ff-golang-server-sdk/logger"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
)

// Cache is the interface for any type that stores keys against a map of fields -> values
// e.g.
//
// some-key-1
//
//	field-1: foobar
//	field-2: fizzbuzz
//
// some-key-2
//
//	field-1: hello-world
type Cache interface {
	// Set sets a value in the cache for a given key and field
	Set(ctx context.Context, key string, value interface{}) error

	// Get gets the value of a field for a given key
	Get(ctx context.Context, key string, value interface{}) error

	// Delete removes a key from the cache
	Delete(ctx context.Context, key string) error

	// Keys returns a list of keys that match the pattern
	Keys(ctx context.Context, key string) ([]string, error)

	// HealthCheck checks cache health
	HealthCheck(ctx context.Context) error
}

// Wrapper wraps a given cache with logic to store features and segments passed from the golang sdk
// it translates them from the sdk representation to our rest representation and buckets them per environment
// this means the proxy can store all the data from all sdk instances in one large cache but to each sdk it appears
// to have it's own unique cache
type Wrapper struct {
	// for now we only support our Memcache
	cache       Cache
	environment string
	logger      log.Logger
	lastUpdate  time.Time
	// we clear out old data for the given environment when we run the first Set instruction
	// this is to verify the sdk has fetched the new data successfully before purging the old data
	firstSet bool
	m        *sync.RWMutex
}

type cacheKey struct {
	kind  string
	name  string
	field string
}

// NewWrapper creates a new Wrapper instance
func NewWrapper(cache Cache, environment string, l log.Logger) *Wrapper {
	l = l.With("component", "CacheWrapper")

	return &Wrapper{
		cache:       cache,
		environment: environment,
		logger:      l,
		firstSet:    true,
		m:           &sync.RWMutex{},
	}
}

func (wrapper *Wrapper) getTime() time.Time {
	return time.Now()
}

// Set sets a new value for a key
func (wrapper *Wrapper) Set(key interface{}, value interface{}) (evicted bool) {
	wrapper.logger = wrapper.logger.With("method", "Set", "key", key, "value", value)

	// on first set delete old data
	wrapper.m.Lock()
	if wrapper.firstSet {
		wrapper.Purge()
		wrapper.firstSet = false
	}
	wrapper.m.Unlock()
	cacheKey, err := wrapper.decodeDTOKey(key)
	if err != nil {
		wrapper.logger.Error("failed to set key, value in cache", "err", err)
		return
	}

	switch cacheKey.kind {
	case dto.KeySegment:
		segmentConfig, ok := value.(rest.Segment)
		if !ok {
			wrapper.logger.Error("failed to cast value in cache to rest.Segment")
			return
		}

		// We used to be able to write the raw bytes of the featureConfig and then
		// unmarshal them in the Proxy to a domain.FeatureConfig. However, because
		// the memoize cache uses reflection we need to use the same type for marshaling
		// and unmarshaling which is why we do this conversion.
		s := domain.Segment(segmentConfig)

		err = wrapper.cache.Set(context.Background(), cacheKey.name, &s)
		if err != nil {
			wrapper.logger.Warn("failed to set key to wrapper cache", "err", err)
			return
		}

	case dto.KeySegments:
		segmentConfig, ok := value.([]rest.Segment)
		if !ok {
			wrapper.logger.Error("failed to cast value in cache to rest.Segment")
			return
		}

		// We used to be able to write the raw bytes of the featureConfig and then
		// unmarshal them in the Proxy to a domain.FeatureConfig. However, because
		// the memoize cache uses reflection we need to use the same type for marshaling
		// and unmarshaling which is why we do this conversion.
		segments := make([]domain.Segment, 0, len(segmentConfig))
		for _, s := range segmentConfig {
			segments = append(segments, domain.Segment(s))
		}

		err = wrapper.cache.Set(context.Background(), cacheKey.name, segments)
		if err != nil {
			wrapper.logger.Warn("failed to set key to wrapper cache", "err", err)
			return
		}

	case dto.KeyFeature:
		featureConfig, ok := value.(rest.FeatureConfig)
		if !ok {
			wrapper.logger.Error("failed to cast value in cache to rest.FeatureConfig")
			return
		}

		// We used to be able to write the raw bytes of the featureConfig and then
		// unmarshal them in the Proxy to a domain.FeatureConfig. However, because
		// the memoize cache uses reflection we need to use the same type for marshaling
		// and unmarshaling which is why we do this conversion.
		ff := domain.FeatureFlag(featureConfig)

		err = wrapper.cache.Set(context.Background(), cacheKey.name, &ff)
		if err != nil {
			wrapper.logger.Warn("failed to set key to wrapper cache", "err", err)
			return
		}

	case dto.KeyFeatures:
		featureConfigs, ok := value.([]rest.FeatureConfig)
		if !ok {
			wrapper.logger.Error("failed to cast value in cache to rest.FeatureConfig")
			return
		}

		// We used to be able to write the raw bytes of the featureConfig and then
		// unmarshal them in the Proxy to a domain.FeatureConfig. However, because
		// the memoize cache uses reflection we need to use the same type for marshaling
		// and unmarshaling which is why we do this conversion.
		features := make([]domain.FeatureFlag, 0, len(featureConfigs))
		for _, f := range featureConfigs {
			features = append(features, domain.FeatureFlag(f))
		}

		err = wrapper.cache.Set(context.Background(), cacheKey.name, features)
		if err != nil {
			wrapper.logger.Warn("failed to set key to wrapper cache", "err", err)
			return
		}

	default:
		wrapper.logger.Error("unexpected type trying to be set")
		return
	}

	wrapper.lastUpdate = wrapper.getTime()

	return
}

// Get looks up a key's value from the cache.
func (wrapper *Wrapper) Get(key interface{}) (value interface{}, ok bool) {
	wrapper.logger = wrapper.logger.With("method", "Get", "key", key)

	cacheKey, err := wrapper.decodeDTOKey(key)
	if err != nil {
		wrapper.logger.Error("failed to get key", "err", err)
		return nil, false
	}

	value, err = wrapper.get(cacheKey)
	if err != nil {
		wrapper.logger.Error("failed to get field for cacheKey", "cacheKeyField", cacheKey.field, "cacheKeyKind", cacheKey.kind, "err", err)
		return nil, false
	}

	return value, true
}

// Keys returns a slice of the keys in the cache
func (wrapper *Wrapper) Keys() []interface{} {
	var keys []interface{}

	// get flag and segment keys
	segmentKeys := wrapper.getKeysByType2(dto.KeySegments)
	if segmentKeys != nil {
		keys = append(keys, segmentKeys...)
	}
	featureKeys := wrapper.getKeysByType(dto.KeyFeatures)
	if featureKeys != nil {
		keys = append(keys, featureKeys...)
	}

	return keys
}

// Remove removes the provided key from the cache.
func (wrapper *Wrapper) Remove(key interface{}) (present bool) {
	wrapper.logger = wrapper.logger.With("method", "Remove", "key", key)

	cacheKey, err := wrapper.decodeDTOKey(key)
	if err != nil {
		wrapper.logger.Error("failed to remove key", "err", err)
		return false
	}

	present = wrapper.Contains(key)
	wrapper.cache.Delete(context.Background(), cacheKey.name)
	wrapper.lastUpdate = wrapper.getTime()
	return present
}

// Updated lastUpdate information
func (wrapper *Wrapper) Updated() time.Time {
	return wrapper.lastUpdate
}

// SetLogger sets the wrappers logger from a logger.Logger
func (wrapper *Wrapper) SetLogger(l logger.Logger) {
	og, ok := l.(*logger.ZapLogger)
	if !ok {
		l.Warnf("failed to set logger in cache wrapper, expected logger to be *logger.ZapLogger, got %T", og)
		return
	}

	sugar := og.Sugar()
	if sugar == nil {
		l.Warn("failed to extract logger")
		return
	}
	wrapper.logger = log.NewStructuredLoggerFromSugar(*sugar).With("component", "CacheWrapper")
}

// Contains checks if a key is in the cache
func (wrapper *Wrapper) Contains(key interface{}) bool {
	_, ok := wrapper.Get(key)
	return ok
}

// Len returns the number of items in the cache.
func (wrapper *Wrapper) Len() int {
	return len(wrapper.Keys())
}

// Purge is used to completely clear the cache.
func (wrapper *Wrapper) Purge() {
	// delete all flags and segments
	wrapper.deleteByType(dto.KeySegments)
	wrapper.deleteByType(dto.KeyFeatures)

	wrapper.lastUpdate = wrapper.getTime()
}

// Resize changes the cache size.
func (wrapper *Wrapper) Resize(size int) (evicted int) {
	wrapper.logger.Warn("Resize method not supported")
	return 0
}

/*
 *  Util functions
 */

func (wrapper *Wrapper) decodeDTOKey(key interface{}) (cacheKey, error) {

	dtoKey, err := convertToDTOKey(key)
	if err != nil {
		return cacheKey{}, fmt.Errorf("couldn't convert key to dto.Key: %s", key)
	}

	keyName, err := wrapper.generateKeyName(dtoKey.Type, dtoKey.Name)
	if err != nil {
		return cacheKey{}, err
	}

	return cacheKey{
		kind:  dtoKey.Type,
		name:  keyName,
		field: dtoKey.Name,
	}, nil
}

// generateKeyName generates the key name from the type and cache environment
func (wrapper *Wrapper) generateKeyName(keyType string, keyName string) (string, error) {
	switch keyType {
	case dto.KeyFeatures:
		return string(domain.NewFeatureConfigsKey(wrapper.environment)), nil
	case dto.KeyFeature:
		return string(domain.NewFeatureConfigKey(wrapper.environment, keyName)), nil
	case dto.KeySegment:
		return string(domain.NewSegmentKey(wrapper.environment, keyName)), nil
	case dto.KeySegments:
		return string(domain.NewSegmentsKey(wrapper.environment)), nil
	default:
		return "", fmt.Errorf("key type not recognised: %s", keyType)
	}
}

func (wrapper *Wrapper) getKeysByType(keyType string) []interface{} {
	wrapper.logger = wrapper.logger.With("method", "getKeysByType", "keyType", keyType)

	keyName, err := wrapper.generateKeyName(keyType, "")
	if err != nil {
		wrapper.logger.Warn("failed to generate key name", "err", err)
		return nil
	}

	results := make([]rest.FeatureConfig, 0)
	err = wrapper.cache.Get(context.Background(), keyName, &results)
	if err != nil {
		wrapper.logger.Warn("failed to GetAll values for keyName", "keyName", keyName, "err", err)
		return nil
	}

	// convert result objects to their dto.Key
	keys := make([]interface{}, 0, len(results))
	for _, f := range results {
		keys = append(keys, dto.Key{
			Type: keyType,
			Name: f.Feature,
		})
	}

	return keys
}

func (wrapper *Wrapper) getKeysByType2(keyType string) []interface{} {
	wrapper.logger = wrapper.logger.With("method", "getKeysByType", "keyType", keyType)

	keyName, err := wrapper.generateKeyName(keyType, "")
	if err != nil {
		wrapper.logger.Warn("failed to generate key name", "err", err)
		return nil
	}

	results := make([]rest.Segment, 0)
	err = wrapper.cache.Get(context.Background(), keyName, &results)
	if err != nil {
		wrapper.logger.Warn("failed to GetAll values for keyName", "keyName", keyName, "err", err)
		return nil
	}

	keys := make([]interface{}, 0, len(results))
	for _, s := range results {
		keys = append(keys, dto.Key{
			Type: keyType,
			Name: s.Identifier,
		})
	}

	return keys
}

func (wrapper *Wrapper) deleteByType(keyType string) {
	wrapper.logger = wrapper.logger.With("method", "deleteByType", "keyType", keyType)

	keyName, err := wrapper.generateKeyName(keyType, "")
	if err != nil {
		wrapper.logger.Warn("skipping purge of key type", "err", err)
		return
	}

	wrapper.cache.Delete(context.Background(), keyName)
	//wrapper.cache.RemoveAll(context.Background(), keyName)
}

func (wrapper *Wrapper) get(key cacheKey) (interface{}, error) {
	switch key.kind {
	case dto.KeyFeatures:
		return wrapper.getFeatureConfigs(key)
	case dto.KeySegments:
		return wrapper.getSegments(key)
	}

	return nil, fmt.Errorf("invalid type %s", key.kind)
}

func (wrapper *Wrapper) getFeatureConfigs(key cacheKey) (interface{}, error) {
	// get FeatureFlag in rest.FeatureConfig format
	var featureConfig []rest.FeatureConfig
	err := wrapper.cache.Get(context.Background(), key.name, &featureConfig)
	if err != nil {
		return nil, err
	}

	return featureConfig, nil
}

func (wrapper *Wrapper) getSegments(key cacheKey) (interface{}, error) {
	var segment []rest.Segment
	// get Segment in domain.Segment format
	err := wrapper.cache.Get(context.Background(), key.name, &segment)
	if err != nil {
		return nil, err
	}

	return segment, nil
}

func convertToDTOKey(key interface{}) (dto.Key, error) {
	myKey, ok := key.(string)
	if !ok {
		dtoKey, ok := key.(dto.Key)
		if !ok {
			return dto.Key{}, fmt.Errorf("couldn't convert key to dto.Key: %s", key)
		}
		return dtoKey, nil
	}

	keyArr := strings.SplitN(myKey, "/", 2)
	if len(keyArr) != 2 {
		return dto.Key{}, fmt.Errorf("couldn't convert key to dto.Key: %s", key)
	}
	dtoKey := dto.Key{
		Type: keyArr[0],
		Name: keyArr[1],
	}
	return dtoKey, nil
}
