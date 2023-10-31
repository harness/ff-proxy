package repository

import (
	"context"
	"errors"

	"github.com/harness/ff-proxy/v2/cache"
	"github.com/harness/ff-proxy/v2/domain"
)

// InventoryRepo is a repository that stores all references to all assets for the key.
type InventoryRepo struct {
	cache cache.Cache
}

// NewInventoryRepo creates new instance of inventory
func NewInventoryRepo(c cache.Cache) InventoryRepo {
	return InventoryRepo{
		cache: c,
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
func (i InventoryRepo) Patch(_ context.Context, _ string, _ map[string]string) error {
	return nil
}

// Cleanup removes all entries for the key which are in the old config but not in the new one
func (i InventoryRepo) Cleanup(ctx context.Context, key string, config []domain.ProxyConfig) error {

	oldAssets, err := i.Get(ctx, key)
	if err != nil {
		return err
	}

	newAssets, err := i.BuildAssetListFromConfig(config)
	if err != nil {
		return err
	}

	// work out assets to delete
	for k := range oldAssets {
		// if the key exists in the new assets we don't want to delete it.
		if _, ok := newAssets[k]; ok {
			delete(oldAssets, k)
		}
	}

	// what's left of old values. we want to delete.
	for key := range oldAssets {
		err := i.cache.Delete(ctx, key)
		if err != nil {
			return err
		}
	}
	// set new inventory.
	return i.Add(ctx, key, newAssets)
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
				//append segments
				inventory[string(domain.NewFeatureConfigsKey(environment))] = empty
				for _, f := range env.FeatureConfigs {
					inventory[string(domain.NewFeatureConfigKey(environment, f.Feature))] = empty
				}
			}
		}
	}

	return inventory, nil
}
