package config

import (
	"errors"
	"io/fs"
	"strings"

	ffproxy "github.com/harness/ff-proxy"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/hash"
)

type config struct {
	Environment  string               `json:"environment"`
	FeatureFlags []domain.FeatureFlag `json:"featureConfig"`
	Targets      []domain.Target      `json:"targets"`
	Segments     []domain.Segment     `json:"segments"`
	Auth         []domain.AuthAPIKey  `json:"auth"`
}

// LocalConfig is a type that can traverse a tree of files and decode
// FeatureFlag, Target and Segment information from them.
type LocalConfig struct {
	config map[string]config
	hasher hash.Hasher
}

// NewLocalConfig creates a new FeatureFlagConfig that loads config from
// the passed FileSystem and directory.
func NewLocalConfig(fs fs.FS) (LocalConfig, error) {
	o := LocalConfig{
		config: make(map[string]config),
		hasher: hash.NewSha256(),
	}

	if err := o.loadConfig(fs); err != nil {
		return LocalConfig{}, err
	}
	return o, nil
}

// loadConfig reads the directory of the filesystem and walks the file tree
// decoding any config files that it finds
func (f LocalConfig) loadConfig(fileSystem fs.FS) error {
	if err := fs.WalkDir(fileSystem, ".", decodeConfigFiles(f.config, fileSystem)); err != nil {
		return err
	}
	return nil
}

// getParentDirFromPath gets the name of the parent directory for a file in a path
func getParentDirFromPath(path string) (string, error) {
	split := strings.SplitAfter(path, "/")
	if len(split) < 2 {
		return "", errors.New("path needs a length of at least 2 to have a parent")
	}

	// Need to remove trailing slash from parent directory after strings split
	return strings.TrimSuffix(split[len(split)-2], "/"), nil
}

// decodeConfigFiles returns a WalkDirFunc that gets called on each file in the
// config directory.
func decodeConfigFiles(c map[string]config, fileSystem fs.FS) fs.WalkDirFunc {
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
				Environment:  strings.TrimPrefix(i.Name(), "env-"),
				FeatureFlags: []domain.FeatureFlag{},
				Targets:      []domain.Target{},
				Segments:     []domain.Segment{},
				Auth:         []domain.AuthAPIKey{},
			}
			return nil
		}

		// Seems like the only way of getting the name of the directory that a
		// file is in is by parsing the path
		env, err := getParentDirFromPath(path)
		if err != nil {
			return nil
		}

		if i.Name() == "feature_config.json" {
			config := c[env]
			if err := ffproxy.DecodeFile(fileSystem, path, &config.FeatureFlags); err != nil {
				return err
			}
			c[env] = config
			return nil
		}

		if i.Name() == "targets.json" {
			config := c[env]
			if err := ffproxy.DecodeFile(fileSystem, path, &config.Targets); err != nil {
				return err
			}
			c[env] = config
			return nil
		}

		if i.Name() == "segments.json" {
			config := c[env]
			if err := ffproxy.DecodeFile(fileSystem, path, &config.Segments); err != nil {
				return err
			}
			c[env] = config
			return nil
		}

		if i.Name() == "auth_config.json" {
			config := c[env]
			if err := ffproxy.DecodeFile(fileSystem, path, &config.Auth); err != nil {
				return err
			}

			c[env] = config
		}
		return nil
	}
}

func (f LocalConfig) FeatureFlag() map[domain.FeatureFlagKey]interface{} {
	result := map[domain.FeatureFlagKey]interface{}{}

	for _, cfg := range f.config {
		if len(cfg.FeatureFlags) > 0 {
			key := domain.NewFeatureConfigsKey(cfg.Environment)
			result[key] = cfg.FeatureFlags
		}

		for i := 0; i < len(cfg.FeatureFlags); i++ {
			ff := cfg.FeatureFlags[i]
			fkey := domain.NewFeatureConfigKey(cfg.Environment, ff.Feature)
			result[fkey] = &ff
		}
	}
	return result
}

// Targets returns the target information from the FeatureFlagConfig in the form
// of a map of domain.TargetKey to slice of domain.Target
func (f LocalConfig) Targets() map[domain.TargetKey]interface{} {
	results := map[domain.TargetKey]interface{}{}

	for _, cfg := range f.config {
		if len(cfg.Targets) > 0 {
			key := domain.NewTargetsKey(cfg.Environment)
			results[key] = cfg.Targets
		}

		for i := 0; i < len(cfg.Targets); i++ {
			t := cfg.Targets[i]
			k := domain.NewTargetKey(cfg.Environment, t.Identifier)
			results[k] = &t
		}
	}
	return results
}

// Segments returns the segment informatino from the FeatureFlagConfig in the form
// of a map of domain.SegmentKey to slice of domain.Segments
func (f LocalConfig) Segments() map[domain.SegmentKey]interface{} {
	results := map[domain.SegmentKey]interface{}{}

	for _, cfg := range f.config {
		if len(cfg.Segments) > 0 {
			key := domain.NewSegmentsKey(cfg.Environment)
			results[key] = cfg.Segments
		}

		for i := 0; i < len(cfg.Segments); i++ {
			s := cfg.Segments[i]
			results[domain.NewSegmentKey(cfg.Environment, s.Identifier)] = &s
		}
	}
	return results
}

// AuthConfig returns the authentication config information
func (f LocalConfig) AuthConfig() map[domain.AuthAPIKey]string {
	results := map[domain.AuthAPIKey]string{}
	for _, cfg := range f.config {
		for _, key := range cfg.Auth {
			results[domain.NewAuthAPIKey(string(key))] = cfg.Environment
		}
	}
	return results
}
