package config

import (
	"errors"
	"io/fs"
	"strings"

	ffproxy "github.com/harness/ff-proxy"
	"github.com/harness/ff-proxy/domain"
)

type config struct {
	Environment   string                 `json:"environment"`
	FeatureConfig []domain.FeatureConfig `json:"featureConfig"`
	Targets       []domain.Target        `json:"targets"`
	Segments      []domain.Segment       `json:"segments"`
	Auth          []domain.AuthAPIKey    `json:"auth"`
}

// LocalConfig is a type that can traverse a tree of files and decode
// FeatureConfig, Target and Segment information from them.
type LocalConfig struct {
	config map[string]config
}

// NewLocalConfig creates a new FeatureFlagConfig that loads config from
// the passed FileSystem and directory.
func NewLocalConfig(fs fs.FS, dir string) (LocalConfig, error) {
	o := LocalConfig{
		config: make(map[string]config),
	}

	if err := o.loadConfig(fs, dir); err != nil {
		return LocalConfig{}, err
	}
	return o, nil
}

// loadConfig reads the directory of the filesystem and walks the file tree
// decoding any config files that it finds
func (f LocalConfig) loadConfig(fileSystem fs.FS, dir string) error {
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
				FeatureConfig: []domain.FeatureConfig{},
				Targets:       []domain.Target{},
				Segments:      []domain.Segment{},
				Auth:          []domain.AuthAPIKey{},
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
			if err := ffproxy.DecodeFile(path, &config.FeatureConfig); err != nil {
				return err
			}
			c[env] = config
			return nil
		}

		if i.Name() == "targets.json" {
			config := c[env]
			if err := ffproxy.DecodeFile(path, &config.Targets); err != nil {
				return err
			}
			c[env] = config
			return nil
		}

		if i.Name() == "segments.json" {
			config := c[env]
			if err := ffproxy.DecodeFile(path, &config.Segments); err != nil {
				return err
			}
			c[env] = config
			return nil
		}

		if i.Name() == "auth_config.json" {
			config := c[env]
			if err := ffproxy.DecodeFile(path, &config.Auth); err != nil {
				return err
			}
			c[env] = config
		}
		return nil
	}
}

// FeatureConfig returns the FeatureConfig information from the FeatureFlagConfig
// in the form of a map of domain.FeatureConfigKeys to slice of domain.FeatureConfig.
// As a part of its logic it adds the Segment information from the FeatureFlagConfig
// to the FeatureConfig type
func (f LocalConfig) FeatureConfig() map[domain.FeatureConfigKey][]domain.FeatureConfig {
	result := map[domain.FeatureConfigKey][]domain.FeatureConfig{}

	for _, cfg := range f.config {
		key := domain.NewFeatureConfigKey(cfg.Environment)

		for i := 0; i < len(cfg.FeatureConfig); i++ {
			fc := &cfg.FeatureConfig[i]

			for _, seg := range cfg.Segments {
				if fc.Segments == nil {
					fc.Segments = make(map[string]domain.Segment)
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
// of a map of domain.TargetKey to slice of domain.Target
func (f LocalConfig) Targets() map[domain.TargetKey][]domain.Target {
	results := map[domain.TargetKey][]domain.Target{}

	for _, cfg := range f.config {
		key := domain.NewTargetKey(cfg.Environment)
		results[key] = cfg.Targets
	}
	return results
}

// Segments returns the segment informatino from the FeatureFlagConfig in the form
// of a map of domain.SegmentKey to slice of domain.Segments
func (f LocalConfig) Segments() map[domain.SegmentKey][]domain.Segment {
	results := map[domain.SegmentKey][]domain.Segment{}

	for _, cfg := range f.config {
		key := domain.NewSegmentKey(cfg.Environment)
		results[key] = cfg.Segments
	}
	return results
}

// AuthConfig returns the authentication config information
func (f LocalConfig) AuthConfig() map[domain.AuthAPIKey]string {
	results := map[domain.AuthAPIKey]string{}
	for _, cfg := range f.config {
		for _, key := range cfg.Auth {
			results[key] = cfg.Environment
		}
	}
	return results
}