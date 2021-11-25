package proxyservice

import (
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/config"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/log"
	"github.com/harness/ff-proxy/repository"
)

type fileSystem struct {
	path string
}

func (f fileSystem) Open(name string) (fs.File, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return file, nil
}

type benchmarkConfig struct {
	auth        map[domain.AuthAPIKey]string
	features    map[domain.FeatureFlagKey][]domain.FeatureFlag
	targets     map[domain.TargetKey][]domain.Target
	segments    map[domain.SegmentKey][]domain.Segment
	numFeatures int
	numTargets  int
	numSegments int
	numAPIKeys  int
}

func getAllBenchmarkConfig() benchmarkConfig {
	dir := fmt.Sprintf("../config/bench-test")
	fileSystem := fileSystem{path: dir}

	lc, err := config.NewLocalConfig(fileSystem, dir)
	if err != nil {
		panic(err)
	}

	auth := lc.AuthConfig()
	features := lc.FeatureFlag()
	segments := lc.Segments()
	targets := lc.Targets()

	return benchmarkConfig{
		auth:     auth,
		features: features,
		segments: segments,
		targets:  targets,
	}

}

func getConfigByEnv(envID string, b *testing.B) benchmarkConfig {
	dir := fmt.Sprintf("../config/bench-test/env-%s", envID)
	fileSystem := fileSystem{path: dir}

	lc, err := config.NewLocalConfig(fileSystem, dir)
	if err != nil {
		b.Fatalf("failed to load config: %s", err)
	}

	auth := lc.AuthConfig()
	features := lc.FeatureFlag()
	segments := lc.Segments()
	targets := lc.Targets()

	return benchmarkConfig{
		auth:        auth,
		features:    features,
		segments:    segments,
		targets:     targets,
		numFeatures: len(features[domain.NewFeatureConfigKey(envID)]),
		numTargets:  len(targets[domain.NewTargetKey(envID)]),
		numSegments: len(segments[domain.NewSegmentKey(envID)]),
	}

}

func setupService(cfg benchmarkConfig, b *testing.B) ProxyService {
	cache := cache.NewMemCache()

	featureRepo, err := repository.NewFeatureFlagRepo(cache, cfg.features)
	if err != nil {
		b.Fatalf("failed to setup FeatureFlagRepo: %s", err)
	}

	segmentRepo, err := repository.NewSegmentRepo(cache, cfg.segments)
	if err != nil {
		b.Fatalf("failed to setup FeatureFlagRepo: %s", err)
	}

	targetRepo, err := repository.NewTargetRepo(cache, cfg.targets)
	if err != nil {
		b.Fatalf("failed to setup FeatureFlagRepo: %s", err)
	}

	authFn := func(key string) (string, error) {
		return "", nil
	}

	return NewService(featureRepo, targetRepo, segmentRepo, authFn, NewFeatureEvaluator(), log.NoOpLogger{})
}

type benchmark struct {
	envID string
	cfg   benchmarkConfig
}

type benchmarks []benchmark

func (b benchmarks) Len() int {
	return len(b)
}

func (b benchmarks) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b benchmarks) Less(i, j int) bool {
	iEnvID := b[i].envID
	jEnvID := b[j].envID

	iFlagKey := domain.NewFeatureConfigKey(iEnvID)
	jFlagKey := domain.NewFeatureConfigKey(jEnvID)

	iFeatures := b[i].cfg.features[iFlagKey]
	jFeatures := b[j].cfg.features[jFlagKey]

	if len(iFeatures) != len(jFeatures) {
		return len(iFeatures) < len(jFeatures)
	}

	iSegKey := domain.NewSegmentKey(iEnvID)
	jSegKey := domain.NewSegmentKey(jEnvID)

	iSegments := b[i].cfg.segments[iSegKey]
	jSegments := b[j].cfg.segments[jSegKey]

	if len(iSegments) != len(jSegments) {
		return len(iSegments) < len(jSegments)
	}

	iTargetKey := domain.NewTargetKey(iEnvID)
	jTargetKey := domain.NewTargetKey(jEnvID)

	iTargets := b[i].cfg.targets[iTargetKey]
	jTargets := b[j].cfg.targets[jTargetKey]

	return len(iTargets) < len(jTargets)
}

func makeBenchmarks() benchmarks {
	var bms benchmarks = []benchmark{}
	cfg := getAllBenchmarkConfig()

	environmets := []string{}
	for key := range cfg.features {
		envID := strings.TrimSuffix(strings.TrimPrefix(string(key), "env-"), "-feature-config")
		environmets = append(environmets, envID)
	}

	for _, env := range environmets {
		b := benchmark{
			envID: env,
			cfg:   cfg,
		}
		bms = append(bms, b)
	}

	sort.Sort(bms)
	return bms
}

func BenchmarkFeatureConfig(b *testing.B) {
	benchmarks := makeBenchmarks()

	for _, bm := range benchmarks {
		bm := bm

		numFeatures := len(bm.cfg.features[domain.NewFeatureConfigKey(bm.envID)])
		numSegments := len(bm.cfg.segments[domain.NewSegmentKey(bm.envID)])
		numTargets := len(bm.cfg.targets[domain.NewTargetKey(bm.envID)])

		name := fmt.Sprintf("env-%s, NumFeatures=%d, NumSegments=%d, NumTargets=%d", bm.envID, numFeatures, numSegments, numTargets)

		cfg := getConfigByEnv(bm.envID, b)
		service := setupService(cfg, b)
		ctx := context.Background()
		req := domain.FeatureConfigRequest{
			EnvironmentID: bm.envID,
		}

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := service.FeatureConfig(ctx, req)
				if err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func BenchmarkFeatureConfigByIdentifier(b *testing.B) {
	benchmarks := makeBenchmarks()

	for _, bm := range benchmarks {
		bm := bm

		numFeatures := len(bm.cfg.features[domain.NewFeatureConfigKey(bm.envID)])
		numSegments := len(bm.cfg.segments[domain.NewSegmentKey(bm.envID)])
		numTargets := len(bm.cfg.targets[domain.NewTargetKey(bm.envID)])

		name := fmt.Sprintf("env-%s, NumFeatures=%d, NumSegments=%d, NumTargets=%d", bm.envID, numFeatures, numSegments, numTargets)

		rand.Seed(time.Now().Unix())
		identifier := fmt.Sprintf("feature-%d", rand.Intn(numFeatures))

		cfg := getConfigByEnv(bm.envID, b)
		service := setupService(cfg, b)
		ctx := context.Background()
		req := domain.FeatureConfigByIdentifierRequest{
			EnvironmentID: bm.envID,
			Identifier:    identifier,
		}

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := service.FeatureConfigByIdentifier(ctx, req)
				if err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func BenchmarkTargetSegments(b *testing.B) {
	benchmarks := makeBenchmarks()

	for _, bm := range benchmarks {
		bm := bm

		numFeatures := len(bm.cfg.features[domain.NewFeatureConfigKey(bm.envID)])
		numSegments := len(bm.cfg.segments[domain.NewSegmentKey(bm.envID)])
		numTargets := len(bm.cfg.targets[domain.NewTargetKey(bm.envID)])

		name := fmt.Sprintf("env-%s, NumFeatures=%d, NumSegments=%d, NumTargets=%d", bm.envID, numFeatures, numSegments, numTargets)

		cfg := getConfigByEnv(bm.envID, b)
		service := setupService(cfg, b)
		ctx := context.Background()
		req := domain.TargetSegmentsRequest{
			EnvironmentID: bm.envID,
		}

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := service.TargetSegments(ctx, req)
				if err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func BenchmarkTargetSegmentsByIdentifier(b *testing.B) {
	benchmarks := makeBenchmarks()

	for _, bm := range benchmarks {
		bm := bm

		numFeatures := len(bm.cfg.features[domain.NewFeatureConfigKey(bm.envID)])
		numSegments := len(bm.cfg.segments[domain.NewSegmentKey(bm.envID)])
		numTargets := len(bm.cfg.targets[domain.NewTargetKey(bm.envID)])

		name := fmt.Sprintf("env-%s, NumFeatures=%d, NumSegments=%d, NumTargets=%d", bm.envID, numFeatures, numSegments, numTargets)

		rand.Seed(time.Now().Unix())
		identifier := fmt.Sprintf("segment-%d", rand.Intn(numSegments))

		cfg := getConfigByEnv(bm.envID, b)
		service := setupService(cfg, b)
		ctx := context.Background()
		req := domain.TargetSegmentsByIdentifierRequest{
			EnvironmentID: bm.envID,
			Identifier:    identifier,
		}

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := service.TargetSegmentsByIdentifier(ctx, req)
				if err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func BenchmarkEvaluations(b *testing.B) {
	benchmarks := makeBenchmarks()

	for _, bm := range benchmarks {
		bm := bm

		numFeatures := len(bm.cfg.features[domain.NewFeatureConfigKey(bm.envID)])
		numSegments := len(bm.cfg.segments[domain.NewSegmentKey(bm.envID)])
		numTargets := len(bm.cfg.targets[domain.NewTargetKey(bm.envID)])

		name := fmt.Sprintf("env-%s, NumFeatures=%d, NumSegments=%d, NumTargets=%d", bm.envID, numFeatures, numSegments, numTargets)

		rand.Seed(time.Now().Unix())
		identifier := fmt.Sprintf("target-%d", rand.Intn(numTargets))

		cfg := getConfigByEnv(bm.envID, b)
		service := setupService(cfg, b)
		ctx := context.Background()
		req := domain.EvaluationsRequest{
			EnvironmentID:    bm.envID,
			TargetIdentifier: identifier,
		}

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := service.Evaluations(ctx, req)
				if err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func BenchmarkEvaluationsByFeature(b *testing.B) {
	benchmarks := makeBenchmarks()

	for _, bm := range benchmarks {
		bm := bm

		numFeatures := len(bm.cfg.features[domain.NewFeatureConfigKey(bm.envID)])
		numSegments := len(bm.cfg.segments[domain.NewSegmentKey(bm.envID)])
		numTargets := len(bm.cfg.targets[domain.NewTargetKey(bm.envID)])

		name := fmt.Sprintf("env-%s, NumFeatures=%d, NumSegments=%d, NumTargets=%d", bm.envID, numFeatures, numSegments, numTargets)

		rand.Seed(time.Now().Unix())
		target := fmt.Sprintf("target-%d", rand.Intn(numTargets))
		feature := fmt.Sprintf("feature-%d", rand.Intn(numFeatures))

		cfg := getConfigByEnv(bm.envID, b)
		service := setupService(cfg, b)
		ctx := context.Background()
		req := domain.EvaluationsByFeatureRequest{
			EnvironmentID:     bm.envID,
			TargetIdentifier:  target,
			FeatureIdentifier: feature,
		}

		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := service.EvaluationsByFeature(ctx, req)
				if err != nil {
					b.Error(err)
				}
			}
		})
	}
}
