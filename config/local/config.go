package local

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	ffproxy "github.com/harness/ff-proxy/v2"
	"github.com/harness/ff-proxy/v2/config"
	"github.com/harness/ff-proxy/v2/domain"
	"github.com/harness/ff-proxy/v2/hash"
)

type configObject struct {
	Environment  string               `json:"environment"`
	FeatureFlags []domain.FeatureFlag `json:"featureConfig"`
	Targets      []domain.Target      `json:"targets"`
	Segments     []domain.Segment     `json:"segments"`
	Auth         []domain.AuthAPIKey  `json:"auth"`
}

// Config is a type that can traverse a tree of files and decode
// FeatureFlag, Target and Segment information from them.
type Config struct {
	config map[string]configObject
	hasher hash.Hasher
}

// NewConfig creates a new FeatureFlagConfig that loads configObject from
// the passed FileSystem and directory.
func NewConfig(fs fs.FS) (Config, error) {
	o := Config{
		config: make(map[string]configObject),
		hasher: hash.NewSha256(),
	}

	if err := o.loadConfig(fs); err != nil {
		return Config{}, err
	}
	return o, nil
}

// Populate populates the repos with the config loaded from the file system
func (c Config) Populate(ctx context.Context, authRepo config.AuthRepo, flagRepo config.FlagRepo, segmentRepo config.SegmentRepo) error {
	var (
		authConfig    []domain.AuthConfig
		flagConfig    []domain.FlagConfig
		segmentConfig []domain.SegmentConfig
	)

	for envID, f := range c.config {
		for _, key := range f.Auth {
			authConfig = append(authConfig, domain.AuthConfig{
				APIKey:        key,
				EnvironmentID: domain.EnvironmentID(envID),
			})
		}

		flagConfig = append(flagConfig, domain.FlagConfig{
			EnvironmentID:  envID,
			FeatureConfigs: f.FeatureFlags,
		})

		segmentConfig = append(segmentConfig, domain.SegmentConfig{
			EnvironmentID: envID,
			Segments:      f.Segments,
		})
	}

	if err := authRepo.Add(ctx, authConfig...); err != nil {
		return fmt.Errorf("failed to add auth config to cache: %s", err)
	}

	if err := flagRepo.Add(ctx, flagConfig...); err != nil {
		return fmt.Errorf("failed to add flag config to cache: %s", err)
	}

	if err := segmentRepo.Add(ctx, segmentConfig...); err != nil {
		return fmt.Errorf("failed to add segment config to cache: %s", err)
	}

	return nil
}

// loadConfig reads the directory of the filesystem and walks the file tree
// decoding any configObject files that it finds
func (c Config) loadConfig(fileSystem fs.FS) error {
	if err := fs.WalkDir(fileSystem, ".", decodeConfigFiles(c.config, fileSystem)); err != nil {
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
// configObject directory.
func decodeConfigFiles(c map[string]configObject, fileSystem fs.FS) fs.WalkDirFunc {
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

			c[i.Name()] = configObject{
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
			cfg := c[env]
			if err := ffproxy.DecodeFile(fileSystem, path, &cfg.FeatureFlags); err != nil {
				return err
			}
			c[env] = cfg
			return nil
		}

		if i.Name() == "targets.json" {
			cfg := c[env]
			if err := ffproxy.DecodeFile(fileSystem, path, &cfg.Targets); err != nil {
				return err
			}
			c[env] = cfg
			return nil
		}

		if i.Name() == "segments.json" {
			cfg := c[env]
			if err := ffproxy.DecodeFile(fileSystem, path, &cfg.Segments); err != nil {
				return err
			}
			c[env] = cfg
			return nil
		}

		if i.Name() == "auth_config.json" {
			cfg := c[env]
			if err := ffproxy.DecodeFile(fileSystem, path, &cfg.Auth); err != nil {
				return err
			}

			c[env] = cfg
		}
		return nil
	}
}
