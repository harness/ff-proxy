package domain

import "context"

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
}

// SegmentRepo is the interface for the SegmentRepository
type SegmentRepo interface {
	Add(ctx context.Context, config ...SegmentConfig) error
	Remove(ctx context.Context, id string) error
}
