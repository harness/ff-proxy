package cache

import (
	"context"
	"encoding"
	"encoding/json"
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
//    field-1: foobar
//    field-2: fizzbuzz
// some-key-2
//    field-1: hello-world
type Cache interface {
	// Set sets a value in the cache for a given key and field
	Set(ctx context.Context, key string, field string, value encoding.BinaryMarshaler) error
	// SetByte sets a value in the cache for a given key and field
	SetByte(ctx context.Context, key string, field string, value []byte) error
	// Get gets the value of a field for a given key
	Get(ctx context.Context, key string, field string, v encoding.BinaryUnmarshaler) error
	// GetByte gets the value of a field for a given key
	GetByte(ctx context.Context, key string, field string) ([]byte, error)
	// GetAll gets all of the fiels and their values for a given key
	GetAll(ctx context.Context, key string) (map[string][]byte, error)
	// RemoveAll removes all the fields and their values for a given key
	RemoveAll(ctx context.Context, key string)
	// Remove removes a field for a given key
	Remove(ctx context.Context, key string, field string)
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

	var val []byte
	switch cacheKey.kind {
	case dto.KeySegment:
		segmentConfig, ok := value.(rest.Segment)
		if !ok {
			wrapper.logger.Error("failed to cast value in cache to rest.Segment")
			return
		}
		val, err = json.Marshal(segmentConfig)
		if err != nil {
			wrapper.logger.Error("failed to marshal segmentConfig", "err", err)
			return
		}
	case dto.KeyFeature:
		featureConfig, ok := value.(rest.FeatureConfig)
		if !ok {
			wrapper.logger.Error("failed to cast value in cache to rest.FeatureConfig")
			return
		}
		val, err = json.Marshal(featureConfig)
		if err != nil {
			wrapper.logger.Error("failed to marshal featureConfig", "err", err)
			return
		}
	default:
		wrapper.logger.Error("unexpected type trying to be set")
		return
	}

	err = wrapper.cache.SetByte(context.Background(), cacheKey.name, cacheKey.field, val)
	if err != nil {
		wrapper.logger.Warn("failed to set key to wrapper cache", "err", err)
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
	segmentKeys := wrapper.getKeysByType(dto.KeySegment)
	if segmentKeys != nil {
		keys = append(keys, segmentKeys...)
	}
	featureKeys := wrapper.getKeysByType(dto.KeyFeature)
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
	wrapper.cache.Remove(context.Background(), cacheKey.name, cacheKey.field)
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
	wrapper.deleteByType(dto.KeySegment)
	wrapper.deleteByType(dto.KeyFeature)

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

	keyName, err := wrapper.generateKeyName(dtoKey.Type)
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
func (wrapper *Wrapper) generateKeyName(keyType string) (string, error) {
	switch keyType {
	case dto.KeyFeature:
		return string(domain.NewFeatureConfigKey(wrapper.environment)), nil
	case dto.KeySegment:
		return string(domain.NewSegmentKey(wrapper.environment)), nil
	default:
		return "", fmt.Errorf("key type not recognised: %s", keyType)
	}
}

func (wrapper *Wrapper) getKeysByType(keyType string) []interface{} {
	wrapper.logger = wrapper.logger.With("method", "getKeysByType", "keyType", keyType)

	var keys []interface{}

	keyName, err := wrapper.generateKeyName(keyType)
	if err != nil {
		wrapper.logger.Warn("failed to generate key name", "err", err)
		return nil
	}

	results, err := wrapper.cache.GetAll(context.Background(), keyName)
	if err != nil {
		wrapper.logger.Warn("failed to GetAll values for keyName", "keyName", keyName, "err", err)
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

func (wrapper *Wrapper) deleteByType(keyType string) {
	wrapper.logger = wrapper.logger.With("method", "deleteByType", "keyType", keyType)

	keyName, err := wrapper.generateKeyName(keyType)
	if err != nil {
		wrapper.logger.Warn("skipping purge of key type", "err", err)
		return
	}

	wrapper.cache.RemoveAll(context.Background(), keyName)
}

func (wrapper *Wrapper) get(key cacheKey) (interface{}, error) {
	switch key.kind {
	case dto.KeyFeature:
		return wrapper.getFeatureConfig(key)
	case dto.KeySegment:
		return wrapper.getSegment(key)
	}

	return nil, fmt.Errorf("invalid type %s", key.kind)
}

func (wrapper *Wrapper) getFeatureConfig(key cacheKey) (interface{}, error) {
	// get FeatureFlag in rest.FeatureConfig format
	var featureConfig = rest.FeatureConfig{}
	val, err := wrapper.cache.GetByte(context.Background(), key.name, key.field)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(val, &featureConfig)
	if err != nil {
		return nil, fmt.Errorf("couldn't cast cached value to rest.FeatureConfig: %s", val)
	}
	// return to sdk in rest.FeatureConfig format
	return featureConfig, nil
}

func (wrapper *Wrapper) getSegment(key cacheKey) (interface{}, error) {
	var segment = rest.Segment{}
	// get Segment in domain.Segment format
	val, err := wrapper.cache.GetByte(context.Background(), key.name, key.field)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(val, &segment)
	if err != nil {
		return nil, fmt.Errorf("couldn't cast cached value to rest.Segment: %s", val)
	}

	// return to sdk in evaluation.Segment format
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
