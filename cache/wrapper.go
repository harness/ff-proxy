package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/harness/ff-golang-server-sdk/rest"

	"github.com/harness/ff-golang-server-sdk/dto"
	"github.com/harness/ff-golang-server-sdk/logger"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

// Cache is the interface for a key value cache
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
	cacheKey, err := wrapper.decodeDTOKey(key)
	if err != nil {
		wrapper.logger.Error("failed to get key", "err", err)
		return nil, false
	}

	value, err = wrapper.get(cacheKey)
	if err != nil {
		if !errors.Is(err, domain.ErrCacheNotFound) {
			wrapper.logger.Error("failed to get field for cacheKey", "cacheKeyField", cacheKey.field, "cacheKeyKind", cacheKey.kind, "err", err)
		}
		return nil, false
	}

	return value, true
}

// Keys returns a slice of the keys in the cache
func (wrapper *Wrapper) Keys() []interface{} {
	var keys []interface{}

	// get flag and segment keys
	segmentKeys := wrapper.getSegmentKeys(dto.KeySegments)
	if segmentKeys != nil {
		keys = append(keys, segmentKeys...)
	}

	featureKeys := wrapper.getFeatureKeys(dto.KeyFeatures)
	if featureKeys != nil {
		keys = append(keys, featureKeys...)
	}

	return keys
}

// Remove removes the provided key from the cache.
func (wrapper *Wrapper) Remove(key interface{}) (present bool) {
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
	wrapper.deleteByType(dto.KeyFeature)
	wrapper.deleteByType(dto.KeySegment)

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

func (wrapper *Wrapper) getFeatureKeys(keyType string) []interface{} {
	ctx := context.Background()

	keyName, err := wrapper.generateKeyName(keyType, "")
	if err != nil {
		wrapper.logger.Warn("failed to generate key name", "err", err)
		return nil
	}

	results := make([]rest.FeatureConfig, 0)
	if err := wrapper.cache.Get(ctx, keyName, &results); err != nil && !errors.Is(err, domain.ErrCacheNotFound) {
		// Warn but we need to continue in case there are any keys for a single feature
		wrapper.logger.Warn("failed to GetAll values for keyName", "keyName", keyName, "err", err)
	}

	keys := make([]interface{}, 0, len(results))

	// If results isn't empty then that means we found a key for a list of features
	// in the cache and we should add it to the keys array
	if len(results) > 0 {
		keys = append(keys, dto.Key{
			Type: keyType,
			Name: keyName,
		})
	}

	// We now need to check for any keys for a single feature and add any to the array. We don't know
	// the name of the feature(s) we're looking for so the only way to do this is with a wildcard search
	// for any keys matching a prefix
	fk, err := wrapper.searchForKeys(ctx, dto.KeyFeature, fmt.Sprintf("env-%s-feature-config-", wrapper.environment))
	if err != nil {
		return keys
	}

	for _, k := range fk {
		keys = append(keys, k)
	}

	return keys
}

func (wrapper *Wrapper) searchForKeys(ctx context.Context, keyType string, prefix string) ([]dto.Key, error) {
	keys, err := wrapper.cache.Keys(ctx, prefix)
	if err != nil {
		return nil, err
	}

	results := make([]dto.Key, 0, len(keys))
	for _, k := range keys {
		results = append(results, dto.Key{
			Type: keyType,
			Name: k,
		})
	}

	return results, nil
}

func (wrapper *Wrapper) getSegmentKeys(keyType string) []interface{} {
	ctx := context.Background()

	keyName, err := wrapper.generateKeyName(keyType, "")
	if err != nil {
		wrapper.logger.Warn("failed to generate key name", "err", err)
		return nil
	}

	results := make([]rest.Segment, 0)
	if err := wrapper.cache.Get(ctx, keyName, &results); err != nil && !errors.Is(err, domain.ErrCacheNotFound) {
		wrapper.logger.Warn("failed to GetAll values for keyName", "keyName", keyName, "err", err)
		return nil
	}

	keys := make([]interface{}, 0, len(results))

	// If results isn't empty then that means we found a key for a list of segments
	// in the cache and we should add it to the keys array
	if len(results) > 0 {
		keys = append(keys, dto.Key{
			Type: keyType,
			Name: keyName,
		})
	}

	// We now need to check for any keys for a single segment and add any to the array. We don't know
	// the name of the segment(s) we're looking for so the only way to do this is with a wildcard search
	// for any keys matching a prefix
	fk, err := wrapper.searchForKeys(ctx, dto.KeySegment, fmt.Sprintf("env-%s-segment-", wrapper.environment))
	if err != nil {
		return keys
	}

	for _, k := range fk {
		keys = append(keys, k)
	}

	return keys
}

func (wrapper *Wrapper) deleteByType(keyType string) {
	if keyType == dto.KeyFeatures || keyType == dto.KeySegments {
		keyName, err := wrapper.generateKeyName(keyType, "")
		if err != nil {
			wrapper.logger.Warn("skipping purge of key type", "err", err)
			return
		}

		wrapper.cache.Delete(context.Background(), keyName)
	}

	keys := []dto.Key{}
	switch keyType {
	case dto.KeyFeature:
		keys, _ = wrapper.searchForKeys(context.Background(), dto.KeyFeature, fmt.Sprintf("env-%s-feature-config-", wrapper.environment))
	case dto.KeySegment:
		keys, _ = wrapper.searchForKeys(context.Background(), dto.KeySegment, fmt.Sprintf("env-%s-segment-", wrapper.environment))
	}

	for _, key := range keys {
		wrapper.cache.Delete(context.Background(), key.Name)
	}
}

func (wrapper *Wrapper) get(key cacheKey) (interface{}, error) {
	switch key.kind {
	case dto.KeyFeature:
		return wrapper.getFeatureConfig(key)
	case dto.KeyFeatures:
		return wrapper.getFeatureConfigs(key)
	case dto.KeySegment:
		return wrapper.getSegment(key)
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

func (wrapper *Wrapper) getFeatureConfig(key cacheKey) (interface{}, error) {
	// get FeatureFlag in rest.FeatureConfig format
	var ff domain.FeatureFlag
	err := wrapper.cache.Get(context.Background(), key.name, &ff)
	if err != nil {
		return nil, err
	}

	return rest.FeatureConfig(ff), nil
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

func (wrapper *Wrapper) getSegment(key cacheKey) (interface{}, error) {
	var segment domain.Segment
	// get Segment in domain.Segment format
	err := wrapper.cache.Get(context.Background(), key.name, &segment)
	if err != nil {
		return nil, err
	}

	return rest.Segment(segment), nil
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
