package cache

import (
	"context"
	"encoding"
	"fmt"
	"time"

	"github.com/harness/ff-golang-server-sdk/dto"
	"github.com/harness/ff-golang-server-sdk/evaluation"
	"github.com/harness/ff-golang-server-sdk/logger"
	"github.com/harness/ff-proxy/domain"
)

// Wrapper wraps a given cache with logic to store features and segments passed from the golang sdk
// it translates them from the sdk representation to our rest representation and buckets them per environment
// this means the proxy can store all the data from all sdk instances in one large cache but to each sdk it appears
// to have it's own unique cache
type Wrapper struct {
	// for now we only support our Memcache
	*MemCache
	environment string
	logger      logger.Logger
	lastUpdate  time.Time
}

type cacheKey struct {
	kind  string
	name  string
	field string
}

// NewWrapper creates a new Wrapper instance
func NewWrapper(memCache *MemCache, environment string, logger logger.Logger) *Wrapper {
	return &Wrapper{
		MemCache:    memCache,
		environment: environment,
		logger:      logger,
	}
}

func (cache *Wrapper) getTime() time.Time {
	return time.Now()
}

// Set sets a new value for a key
func (cache *Wrapper) Set(key interface{}, value interface{}) (evicted bool) {
	cacheKey, err := cache.decodeDTOKey(key)
	if err != nil {
		cache.logger.Errorf("Set failed: %s", err)
		return
	}

	domainValue, err := cache.convertEvaluationToDomain(cacheKey.kind, value)
	if err != nil {
		cache.logger.Errorf("Set failed: %s", err)
		return
	}

	err = cache.MemCache.Set(context.Background(), cacheKey.name, cacheKey.field, domainValue)
	if err != nil {
		cache.logger.Warnf("Error setting key %s to cache with value %s: %s", key, value, err)
		return
	}

	cache.lastUpdate = cache.getTime()

	return
}

// Get looks up a key's value from the cache.
func (cache *Wrapper) Get(key interface{}) (value interface{}, ok bool) {
	cacheKey, err := cache.decodeDTOKey(key)
	if err != nil {
		cache.logger.Errorf("Get failed: %s", err)
		return nil, false
	}

	value, err = cache.get(cacheKey)
	if err != nil {
		cache.logger.Errorf("Couldn't get field %s of type %s because %s", cacheKey.field, cacheKey.kind, err)
		return nil, false
	}

	return value, true
}

// Keys returns a slice of the keys in the cache
func (cache *Wrapper) Keys() []interface{} {
	var keys []interface{}

	// get flag and segment keys
	segmentKeys := cache.getKeysByType(dto.KeySegment)
	if segmentKeys != nil {
		keys = append(keys, segmentKeys...)
	}
	featureKeys := cache.getKeysByType(dto.KeyFeature)
	if featureKeys != nil {
		keys = append(keys, featureKeys...)
	}

	return keys
}

// Remove removes the provided key from the cache.
func (cache *Wrapper) Remove(key interface{}) (present bool) {
	cacheKey, err := cache.decodeDTOKey(key)
	if err != nil {
		cache.logger.Errorf("Remove failed: %s", err)
		return false
	}

	present = cache.Contains(key)
	cache.MemCache.Remove(context.Background(), cacheKey.name, cacheKey.field)
	cache.lastUpdate = cache.getTime()
	return present
}

// Updated lastUpdate information
func (cache *Wrapper) Updated() time.Time {
	return cache.lastUpdate
}

// SetLogger set logger
func (cache *Wrapper) SetLogger(logger logger.Logger) {
	cache.logger = logger
}

// Contains checks if a key is in the cache
func (cache *Wrapper) Contains(key interface{}) bool {
	_, ok := cache.Get(key)
	return ok
}

// Len returns the number of items in the cache.
func (cache *Wrapper) Len() int {
	return len(cache.Keys())
}

// Purge is used to completely clear the cache.
func (cache *Wrapper) Purge() {
	// delete all flags and segments
	cache.deleteByType(dto.KeySegment)
	cache.deleteByType(dto.KeyFeature)

	cache.lastUpdate = cache.getTime()
}

// Resize changes the cache size.
func (cache *Wrapper) Resize(size int) (evicted int) {
	cache.logger.Warn("Resize method not supported")
	return 0
}

/*
 *  Util functions
 */

func (cache *Wrapper) decodeDTOKey(key interface{}) (cacheKey, error) {
	// decode key
	dtoKey, ok := key.(dto.Key)
	if !ok {
		return cacheKey{}, fmt.Errorf("couldn't convert key to dto.Key: %s", key)
	}

	keyName, err := cache.generateKeyName(dtoKey.Type)
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
func (cache *Wrapper) generateKeyName(keyType string) (string, error) {
	switch keyType {
	case dto.KeyFeature:
		return string(domain.NewFeatureConfigKey(cache.environment)), nil
	case dto.KeySegment:
		return string(domain.NewSegmentKey(cache.environment)), nil
	default:
		return "", fmt.Errorf("key type not recognised: %s", keyType)
	}
}

// generateValue converts the data being cached by the sdk to it's appropriate internal type i.e. domain.FeatureConfig
func (cache *Wrapper) convertEvaluationToDomain(keyType string, value interface{}) (encoding.BinaryMarshaler, error) {
	switch keyType {
	case dto.KeyFeature:
		featureConfig, ok := value.(evaluation.FeatureConfig)
		if !ok {
			return &domain.FeatureConfig{}, fmt.Errorf("couldn't convert to evaluation.FeatureConfig")
		}

		return domain.ConvertEvaluationFeatureConfig(featureConfig), nil
	case dto.KeySegment:
		segmentConfig, ok := value.(evaluation.Segment)
		if !ok {
			return &domain.Segment{}, fmt.Errorf("couldn't convert to evaluation.Segment")
		}
		return domain.ConvertEvaluationSegment(segmentConfig), nil
	default:
		return nil, fmt.Errorf("key type not recognised: %s", keyType)
	}
}

func (cache *Wrapper) getKeysByType(keyType string) []interface{} {
	var keys []interface{}

	keyName, err := cache.generateKeyName(keyType)
	if err != nil {
		cache.logger.Warnf(err.Error())
		return nil
	}

	results, err := cache.MemCache.GetAll(context.Background(), keyName)
	if err != nil {
		cache.logger.Warnf("Couldn't fetch results for %s: %s", keyName, err)
		return nil
	}

	// convert result objects to their dto.Key
	for key := range results {
		keys = append(keys, dto.Key{
			Type: keyType,
			Name: key,
		})
	}

	return keys
}

func (cache *Wrapper) deleteByType(keyType string) {
	keyName, err := cache.generateKeyName(keyType)
	if err != nil {
		cache.logger.Warnf("skipping purge of key type %s: %s", keyType, err)
		return
	}

	cache.MemCache.RemoveAll(context.Background(), keyName)
}

func (cache *Wrapper) get(key cacheKey) (interface{}, error) {
	switch key.kind {
	case dto.KeyFeature:
		return cache.getFeatureConfig(key)
	case dto.KeySegment:
		return cache.getSegment(key)
	}

	return nil, fmt.Errorf("invalid type %s", key.kind)
}

func (cache *Wrapper) getFeatureConfig(key cacheKey) (interface{}, error) {
	var val encoding.BinaryUnmarshaler = &domain.FeatureConfig{}
	// get FeatureConfig in domain.FeatureConfig format
	err := cache.MemCache.Get(context.Background(), key.name, key.field, val)
	if err != nil {
		return nil, err
	}
	featureConfig, ok := val.(*domain.FeatureConfig)
	if !ok {
		return nil, fmt.Errorf("couldn't cast cached value to domain.FeatureConfig: %s", val)
	}

	// return to sdk in evaluation.FeatureConfig format
	return *domain.ConvertDomainFeatureConfig(*featureConfig), nil
}

func (cache *Wrapper) getSegment(key cacheKey) (interface{}, error) {
	var val encoding.BinaryUnmarshaler = &domain.Segment{}
	// get Segment in domain.Segment format
	err := cache.MemCache.Get(context.Background(), key.name, key.field, val)
	if err != nil {
		return nil, err
	}
	segment, ok := val.(*domain.Segment)
	if !ok {
		return nil, fmt.Errorf("couldn't cast cached value to domain.Segment: %s", val)
	}

	// return to sdk in evaluation.Segment format
	return domain.ConvertDomainSegment(*segment), nil
}
