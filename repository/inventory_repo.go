package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/log"
)

const (
	patchVariant   = "patch"
	deleteVariant  = "delete"
	createVariant  = "create"
	segmentVariant = "segment-"
	featureVariant = "feature-config-"
)

var (
	featureConfigRegEx = regexp.MustCompile(`env-([a-zA-Z0-9-_]+)-feature-config-([a-zA-Z0-9-_]+)`)
	segmentConfigRegEx = regexp.MustCompile(`env-([a-zA-Z0-9-]+)-segment-([a-zA-Z0-9-_]+)`)
)

// InventoryRepo is a repository that stores all references to all assets for the key.
type InventoryRepo struct {
	log   log.Logger
	cache cache.Cache
}

// NewInventoryRepo creates new instance of inventory
func NewInventoryRepo(c cache.Cache, l log.Logger) InventoryRepo {
	l = l.With("component", "InventoryRepo")
	return InventoryRepo{
		cache: c,
		log:   l,
	}
}

// Add sets the inventory for proxy config - list of assets for the key.
func (i InventoryRepo) Add(ctx context.Context, key string, assets map[string]string) error {
	return i.cache.Set(ctx, string(domain.NewKeyInventory(key)), assets)
}

func (i InventoryRepo) Remove(_ context.Context, _ string) error {
	return nil
}
func (i InventoryRepo) Get(ctx context.Context, key string) (map[string]string, error) {
	var inventory map[string]string
	err := i.cache.Get(ctx, string(domain.NewKeyInventory(key)), &inventory)
	if err != nil && !errors.Is(err, domain.ErrCacheNotFound) {
		return inventory, err
	}
	return inventory, nil
}

func (i InventoryRepo) Patch(ctx context.Context, key string, updateInventory func(assets map[string]string) (map[string]string, error)) error {
	oldAssets, err := i.Get(ctx, key)
	if err != nil {
		return err
	}
	//logic is different for every case.
	newAssets, err := updateInventory(oldAssets)
	if err != nil {
		return err
	}
	return i.Add(ctx, key, newAssets)
}

// Cleanup removes all entries for the key which are in the old config but not in the new one
func (i InventoryRepo) Cleanup(ctx context.Context, key string, config []domain.ProxyConfig) ([]domain.SSEMessage, error) {

	oldAssets, err := i.Get(ctx, key)
	if err != nil {
		return []domain.SSEMessage{}, err
	}

	newAssets, err := i.BuildAssetListFromConfig(config)
	if err != nil {
		return []domain.SSEMessage{}, err
	}

	//work out differences.
	assets := diffAssets(oldAssets, newAssets)
	notifications := i.BuildNotifications(assets)

	// work out assets to delete
	for k := range oldAssets {
		// if the key exists in the new assets we don't want to delete it.
		if _, ok := newAssets[k]; ok {
			delete(oldAssets, k)
		}
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 1000)
	errChan := make(chan error)

	// what's left of old values. we want to delete.
	for key := range oldAssets {
		wg.Add(1)
		go func(k string) {
			defer func() {
				wg.Done()
				<-semaphore
			}()
			semaphore <- struct{}{}
			errChan <- i.cache.Delete(ctx, k)
		}(key)
	}

	go func() {
		wg.Wait()
		close(semaphore)
		close(errChan)
	}()

	for e := range errChan {
		if e != nil {
			return []domain.SSEMessage{}, e
		}
	}

	if err := i.removeOldKeyData(ctx, key); err != nil {
		return []domain.SSEMessage{}, fmt.Errorf("failed to remove old key data: %w", err)
	}

	// set new inventory.
	err = i.Add(ctx, key, newAssets)
	if err != nil {
		return []domain.SSEMessage{}, err
	}
	return notifications, err
}

func (i InventoryRepo) removeOldKeyData(ctx context.Context, key string) error {
	wildcardKey := domain.NewKeyInventory("*")
	excludeKey := domain.NewKeyInventory(key)

	res, err := i.cache.Scan(ctx, string(wildcardKey))
	if err != nil {
		return err
	}

	delete(res, string(excludeKey))

	for k := range res {
		var oldAssets map[string]string
		err := i.cache.Get(ctx, k, &oldAssets)
		if err != nil {
			return fmt.Errorf("failed to get stale assets for inventory key %q: %s", k, err)
		}

		if err := i.removeAssets(ctx, oldAssets); err != nil {
			return fmt.Errorf("failed to remove stale assets for inventory key %q: %w", k, err)
		}

		// Once we've removed all the assets associated with this key we should remove
		// the key itself so that we don't fetch it the next time we start up
		if err := i.cache.Delete(ctx, k); err != nil {
			return fmt.Errorf("failed to remove stale inventory key %q: %w", k, err)
		}
	}

	return nil
}

func (i InventoryRepo) removeAssets(ctx context.Context, assets map[string]string) error {
	var (
		wg        = &sync.WaitGroup{}
		errChan   = make(chan error)
		semaphore = make(chan struct{}, 1000)
	)

	// what's left of old values. we want to delete.
	for key := range assets {
		wg.Add(1)
		go func(k string) {
			defer func() {
				wg.Done()
				<-semaphore
			}()
			semaphore <- struct{}{}
			errChan <- i.cache.Delete(ctx, k)
		}(key)
	}

	go func() {
		wg.Wait()
		close(semaphore)
		close(errChan)
	}()

	for e := range errChan {
		if e != nil {
			return e
		}
	}

	return nil
}

func diffAssets(oldMap, newMap map[string]string) domain.Assets {
	deleted := make(map[string]string)
	created := make(map[string]string)
	patched := make(map[string]string)

	// Check elements in old but not in new
	for key, value := range oldMap {
		if newValue, exists := newMap[key]; !exists || newValue != value {
			deleted[key] = value
		} else {
			patched[key] = value
		}
	}

	// Check elements in new but not in old
	for key, value := range newMap {
		if _, exists := oldMap[key]; !exists {
			created[key] = value
		}
	}
	return domain.Assets{
		Deleted: deleted,
		Created: created,
		Patched: patched,
	}
}

// BuildAssetListFromConfig returns the list of keys for all assets associated with this proxyKey
func (i InventoryRepo) BuildAssetListFromConfig(config []domain.ProxyConfig) (map[string]string, error) {

	empty := ""
	inventory := make(map[string]string)

	for _, cfg := range config {
		for _, env := range cfg.Environments {
			environment := env.ID.String()
			if len(env.APIKeys) > 0 {
				inventory[string(domain.NewAPIConfigsKey(environment))] = empty
				for _, apiKey := range env.APIKeys {
					inventory[string(domain.NewAuthAPIKey(apiKey))] = empty
				}
			}
			if len(env.FeatureConfigs) > 0 {
				inventory[string(domain.NewFeatureConfigsKey(environment))] = empty
				for _, f := range env.FeatureConfigs {
					inventory[string(domain.NewFeatureConfigKey(environment, f.Feature))] = empty
				}
			}

			if len(env.Segments) > 0 {
				inventory[string(domain.NewSegmentsKey(environment))] = empty
				for _, s := range env.Segments {
					inventory[string(domain.NewSegmentKey(environment, s.Name))] = empty
				}
			}
		}
	}

	return inventory, nil
}

// KeyExists check if the given key exists in cache.
func (i InventoryRepo) KeyExists(ctx context.Context, key string) bool {
	var val interface{}
	err := i.cache.Get(ctx, key, &val)
	if err != nil && errors.Is(err, domain.ErrCacheNotFound) {
		return false
	}
	return true
}

// GetKeysForEnvironment get the map of keys for environment
func (i InventoryRepo) GetKeysForEnvironment(ctx context.Context, env string) (map[string]string, error) {
	scan, err := i.cache.Scan(ctx, env)
	if err != nil {
		return scan, err
	}
	return scan, nil
}

func (i InventoryRepo) BuildNotifications(assets domain.Assets) []domain.SSEMessage {
	var events []domain.SSEMessage
	events = append(events, i.getDeleteEvents(assets.Deleted)...)
	events = append(events, i.getCreateEvents(assets.Created)...)
	// TODO: this currently sends patch notification for all flags
	// regardless weather they have changed or not
	events = append(events, i.getPatchEvents(assets.Patched)...)
	return events
}

func (i InventoryRepo) getDeleteEvents(m map[string]string) []domain.SSEMessage {
	res := make([]domain.SSEMessage, 0, len(m))
	if m == nil {
		return []domain.SSEMessage{}
	}
	for k := range m {
		if strings.Contains(k, featureVariant) {
			res = append(res, i.parseFlagEntry(k, deleteVariant))
		}
		if strings.Contains(k, segmentVariant) {
			res = append(res, i.parseSegmentEntry(k, deleteVariant))
		}
	}
	return res
}

func (i InventoryRepo) getCreateEvents(m map[string]string) []domain.SSEMessage {
	res := make([]domain.SSEMessage, 0, len(m))
	if m == nil {
		return []domain.SSEMessage{}
	}
	for k := range m {
		if strings.Contains(k, featureVariant) {
			res = append(res, i.parseFlagEntry(k, createVariant))
		}
		if strings.Contains(k, segmentVariant) {
			res = append(res, i.parseSegmentEntry(k, createVariant))
		}
	}
	return res
}

func (i InventoryRepo) getPatchEvents(m map[string]string) []domain.SSEMessage {
	res := make([]domain.SSEMessage, 0, len(m))
	if m == nil {
		return []domain.SSEMessage{}
	}
	for k := range m {
		if strings.Contains(k, featureVariant) {
			res = append(res, i.parseFlagEntry(k, patchVariant))
		}
		if strings.Contains(k, segmentVariant) {
			res = append(res, i.parseSegmentEntry(k, patchVariant))
		}
	}
	return res
}

func (i InventoryRepo) parseFlagEntry(flagString, variant string) domain.SSEMessage {
	env, id, err := parseFlagString(flagString)
	if err != nil {
		i.log.Error("err", err)
		return domain.SSEMessage{}
	}
	return domain.SSEMessage{
		Domain:      "flag",
		Event:       variant,
		Identifier:  id,
		Environment: env,
		Version:     0,
	}
}
func (i InventoryRepo) parseSegmentEntry(segmentString, variant string) domain.SSEMessage {
	env, id, err := parseSegmentString(segmentString)
	if err != nil {
		i.log.Error("err", err)
		return domain.SSEMessage{}
	}
	return domain.SSEMessage{
		Domain:      "target-segment",
		Event:       variant,
		Identifier:  id,
		Environment: env,
		Version:     0,
	}
}

func parseFlagString(flagString string) (string, string, error) {
	match := featureConfigRegEx.FindStringSubmatch(flagString)
	if len(match) == 3 {
		return match[1], match[2], nil
	}
	return "", "", errors.New("invalid flag string format")
}

func parseSegmentString(segmentString string) (string, string, error) {
	match := segmentConfigRegEx.FindStringSubmatch(segmentString)
	if len(match) == 3 {
		return match[1], match[2], nil
	}
	return "", "", errors.New("invalid flag string format")
}
