package domain

import "context"

// KeyRepo the interface for keyRepository.
type InventoryRepo interface {
	Add(ctx context.Context, key string, assets map[string]string) error
	Remove(ctx context.Context, key string) error
	Get(ctx context.Context, key string) (map[string]string, error)
	Patch(ctx context.Context, key string, assets map[string]string) error
	BuildAssetListFromConfig(config []ProxyConfig) (map[string]string, error)
	Cleanup(ctx context.Context, key string, config []ProxyConfig) error
}

// AuthRepo is the interface for the AuthRepository
type AuthRepo interface {
	Add(ctx context.Context, config ...AuthConfig) error
	AddAPIConfigsForEnvironment(ctx context.Context, envID string, apiKeys []string) error
	Remove(ctx context.Context, id []string) error
	RemoveAllKeysForEnvironment(ctx context.Context, envID string) error
	PatchAPIConfigForEnvironment(ctx context.Context, envID, apikey, action string) error
}

// FlagRepo is the interface for the FlagRepository
type FlagRepo interface {
	Add(ctx context.Context, config ...FlagConfig) error
	Remove(ctx context.Context, envID, id string) error
	RemoveAllFeaturesForEnvironment(ctx context.Context, id string) error
	GetFeatureConfigForEnvironment(ctx context.Context, envID string) ([]FeatureFlag, bool)
}

// SegmentRepo is the interface for the SegmentRepository
type SegmentRepo interface {
	Add(ctx context.Context, config ...SegmentConfig) error
	Remove(ctx context.Context, envID, id string) error
	RemoveAllSegmentsForEnvironment(ctx context.Context, id string) error
	GetSegmentsForEnvironment(ctx context.Context, envID string) ([]Segment, bool)
}
