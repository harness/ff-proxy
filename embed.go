package ffproxy

import "embed"

var (
	// DefaultConfig embeds the default config directory and the env directories
	// that we care about reading configuration from
	//go:embed config/env-*
	DefaultConfig embed.FS
)

const (
	// DefaultConfigDir is the name of the default directory where the files for
	// side loading FeatureFlagConfig live
	DefaultConfigDir = "config"
)
