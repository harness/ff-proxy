package domain

import "context"

// AuthRepo is the interface for the AuthRepository
type AuthRepo interface {
	Add(ctx context.Context, config ...AuthConfig) error
	Remove(ctx context.Context, id []string) error
}

// FlagRepo is the interface for the FlagRepository
type FlagRepo interface {
	Add(ctx context.Context, config ...FlagConfig) error
	Remove(ctx context.Context, id string) error
}

// SegmentRepo is the interface for the SegmentRepository
type SegmentRepo interface {
	Add(ctx context.Context, config ...SegmentConfig) error
	Remove(ctx context.Context, id string) error
}
