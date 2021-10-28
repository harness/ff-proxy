package ffproxy

import (
	"embed"
	"errors"
	"io/fs"
	"strings"

	"github.com/harness/ff-proxy/gen"
)

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

type config struct {
	Environment   string           `json:"environment"`
	FeatureConfig []*FeatureConfig `json:"featureConfig"`
	Targets       []*gen.Target    `json:"targets"`
	Segments      []*gen.Segment   `json:"segments"`
}

// FeatureFlagConfig is a type that can traverse a tree of files and decode
// FeatureConfig, Target and Segment information from them.
type FeatureFlagConfig struct {
	config map[string]config
}

// NewFeatureFlagConfig creates a new FeatureFlagConfig that loads config from
// the passed FileSystem and directory.
func NewFeatureFlagConfig(fs embed.FS, dir string) (FeatureFlagConfig, error) {
	o := FeatureFlagConfig{
		config: make(map[string]config),
	}

	if err := o.loadConfig(fs, dir); err != nil {
		return FeatureFlagConfig{}, err
	}
	return o, nil
}

// loadConfig reads the directory of the filesystem and walks the file tree
// decoding any config files that it finds
func (f FeatureFlagConfig) loadConfig(fileSystem embed.FS, dir string) error {
	if err := fs.WalkDir(fileSystem, dir, decodeConfigFiles(f.config)); err != nil {
		return err
	}
	return nil
}

// getParentDirFromPath gets the name of the parent directory for a file in a path
func getParentDirFromPath(path string) (string, error) {
	split := strings.SplitAfter(path, "/")
	if len(split) <= 2 {
		return "", errors.New("path needs a length of at least 2 to have a parent")
	}

	// Need to remove trailing slash from parent directory after strings split
	return strings.TrimSuffix(split[len(split)-2], "/"), nil
}

// decodeConfigFiles returns a WalkDirFunc that gets called on each file in the
// config directory.
func decodeConfigFiles(c map[string]config) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		i, err := d.Info()
		if err != nil {
			return err
		}

		if i.IsDir() {
			if !strings.Contains(i.Name(), "env-") {
				return nil
			}

			c[i.Name()] = config{
				Environment:   strings.TrimPrefix(i.Name(), "env-"),
				FeatureConfig: []*FeatureConfig{},
				Targets:       []*gen.Target{},
				Segments:      []*gen.Segment{},
			}
			return nil
		}

		// Seems like the only way of getting the name of the directory that a
		// file is in is by parsing the path
		env, err := getParentDirFromPath(path)
		if err != nil {
			return err
		}

		if i.Name() == "feature_config.json" {
			config := c[env]
			if err := DecodeFile(path, &config.FeatureConfig); err != nil {
				return err
			}
			c[env] = config
			return nil
		}

		if i.Name() == "targets.json" {
			config := c[env]
			if err := DecodeFile(path, &config.Targets); err != nil {
				return err
			}
			c[env] = config
			return nil
		}

		if i.Name() == "segments.json" {
			config := c[env]
			if err := DecodeFile(path, &config.Segments); err != nil {
				return err
			}
			c[env] = config
			return nil
		}
		return nil
	}
}

// FeatureConfig returns the FeatureConfig information from the FeatureFlagConfig
// in the form of a map of featureConfigKeys to []FeatureConfig. As a part of
// its logic it adds the Segment information from the FeatureFlagConfig to the
// FeatureConfig type
func (f FeatureFlagConfig) FeatureConfig() map[FeatureConfigKey][]*FeatureConfig {
	result := map[FeatureConfigKey][]*FeatureConfig{}

	for _, cfg := range f.config {
		key := NewFeatureConfigKey(cfg.Environment)

		for _, fc := range cfg.FeatureConfig {
			for _, seg := range cfg.Segments {
				if fc.Segments == nil {
					fc.Segments = make(map[string]*gen.Segment)
				}

				if _, ok := fc.Segments[seg.Identifier]; !ok {
					fc.Segments[seg.Identifier] = seg
				}
			}
		}
		result[key] = cfg.FeatureConfig
	}
	return result
}

// Targets returns the target information from the FeatureFlagConfig in the form
// of a map of targetIdentifer to Target
func (f FeatureFlagConfig) Targets() map[TargetKey][]*gen.Target {
	results := map[TargetKey][]*gen.Target{}

	for _, cfg := range f.config {
		key := NewTargetKey(cfg.Environment)
		results[key] = cfg.Targets
	}
	return results
}
