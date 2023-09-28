package config

import (
	"context"

	"github.com/harness/ff-proxy/v2/domain"
)

type AuthRepo interface {
	Add(ctx context.Context, config ...domain.AuthConfig) error
}

type FlagRepo interface {
	Add(ctx context.Context, config ...domain.FlagConfig) error
}

type SegmentRepo interface {
	Add(ctx context.Context, config ...domain.SegmentConfig) error
}

// Config defines the interface for populating repositories with configuration data
type Config interface {
	// Populate populates the repos with the config
	Populate(ctx context.Context, authRepo AuthRepo, flagRepo FlagRepo, segmentRepo SegmentRepo) error
}
