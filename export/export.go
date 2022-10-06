package export

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/repository"
)

var readmeTemplate = `
# Env-%s

Env name: %s

Number of Features: %d

Number of Targets: %d

Number of Segments: %d

Generated at: %s
`

const (
	createFilePermissionLevel = 0644
	createDirPermissionLevel  = 0755
)

// OfflineConfig is a struct containing all the offline config to be exported for an environment
type OfflineConfig struct {
	EnvironmentID string
	APIKeys       []string
	Targets       []domain.Target
	Features      []domain.FeatureFlag
	Segments      []domain.Segment
}

// Service is the export service implementation
type Service struct {
	logger      log.Logger
	featureRepo repository.FeatureFlagRepo
	targetRepo  repository.TargetRepo
	segmentRepo repository.SegmentRepo
	authRepo    repository.AuthRepo
	authConfig  map[domain.AuthAPIKey]string
	configDir   string
}

// NewService creates and returns an ExportService
func NewService(logger log.StructuredLogger, featureRepo repository.FeatureFlagRepo, targetRepo repository.TargetRepo,
	segmentRepo repository.SegmentRepo, authRepo repository.AuthRepo, authConfig map[domain.AuthAPIKey]string, configDir string) Service {
	l := logger.With("component", "ExportService")
	return Service{
		logger:      l,
		featureRepo: featureRepo,
		targetRepo:  targetRepo,
		segmentRepo: segmentRepo,
		authRepo:    authRepo,
		authConfig:  authConfig,
		configDir:   configDir,
	}
}

// Persist saves all config to disk
func (s Service) Persist(ctx context.Context) error {
	configMap := map[string]OfflineConfig{}
	for hashedKey, env := range s.authConfig {
		// If we haven't got a config for the env yet lets initialise one and
		// add it to the map
		if _, ok := configMap[env]; !ok {
			features, _ := s.featureRepo.Get(ctx, domain.NewFeatureConfigKey(env))
			targets, _ := s.targetRepo.Get(ctx, domain.NewTargetKey(env))
			segments, _ := s.segmentRepo.Get(ctx, domain.NewSegmentKey(env))

			config := OfflineConfig{
				EnvironmentID: env,
				APIKeys:       []string{string(hashedKey)},
				Targets:       targets,
				Features:      features,
				Segments:      segments,
			}
			configMap[env] = config

			continue
		}
		c := configMap[env]
		c.APIKeys = append(c.APIKeys, string(hashedKey))
		configMap[env] = c
	}

	// make config directory
	os.Mkdir(s.configDir, createDirPermissionLevel)

	for environment, config := range configMap {
		dirName := fmt.Sprintf("%s/env-%s", s.configDir, environment)

		if len(config.APIKeys) == 0 {
			continue
		}

		if err := os.MkdirAll(dirName, createDirPermissionLevel); err != nil {
			return fmt.Errorf("failed to create directory %q: %s", dirName, err)
		}

		authFilename := fmt.Sprintf("%s/auth_config.json", dirName)
		if err := saveConfig(authFilename, config.APIKeys); err != nil {
			return fmt.Errorf("failed to save auth config: %s", err)
		}

		s.logger.Info("writing targets", "count", len(config.Targets))
		targetFilename := fmt.Sprintf("%s/targets.json", dirName)
		if err := saveConfig(targetFilename, config.Targets); err != nil {
			return fmt.Errorf("failed to save target config: %s", err)
		}

		s.logger.Info("writing features", "count", len(config.Features))
		featureFilename := fmt.Sprintf("%s/feature_config.json", dirName)
		if err := saveConfig(featureFilename, config.Features); err != nil {
			return fmt.Errorf("failed to save feature config: %s", err)
		}

		s.logger.Info("writing segments", "count", len(config.Segments))
		segmentsFilename := fmt.Sprintf("%s/segments.json", dirName)
		if err := saveConfig(segmentsFilename, config.Segments); err != nil {
			return fmt.Errorf("failed to save segment config: %s", err)
		}

		readme, err := os.OpenFile(fmt.Sprintf("%s/README.md", dirName), os.O_CREATE|os.O_WRONLY, createFilePermissionLevel)
		if err != nil {
			readme.Close()
			return fmt.Errorf("failed to open README: %s", err)
		}

		var envName string
		if len(config.Features) > 0 {
			envName = config.Features[0].Environment
		}

		_, err = io.WriteString(readme, fmt.Sprintf(readmeTemplate, environment, envName, len(config.Features), len(config.Targets), len(config.Segments), time.Now().Format("2006-01-02 15:04:05")))
		if err != nil {
			return fmt.Errorf("failed writing to readme: %s", err)
		}
	}

	s.logger.Info("Exported config successfully")

	return nil
}

func saveConfig(filename string, v interface{}) error {
	f, err := os.Create(filename)

	if err != nil {
		return fmt.Errorf("failed to open file: %s", err)
	}

	enc := json.NewEncoder(f)
	if err := enc.Encode(v); err != nil {
		f.Close()
		return fmt.Errorf("failed to write to file: %s", err)
	}

	return f.Close()
}
