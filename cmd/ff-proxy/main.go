package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	ffproxy "github.com/harness/ff-proxy"
	"github.com/harness/ff-proxy/cache"
	"github.com/harness/ff-proxy/domain"
	"github.com/harness/ff-proxy/repository"
)

var (
	offline bool
)

func init() {
	flag.BoolVar(&offline, "offline", false, "enables side loading of data from config dir")
	flag.Parse()
}

func main() {
	var (
		featureConfig map[domain.FeatureConfigKey][]domain.FeatureConfig
		targetConfig  map[domain.TargetKey][]domain.Target
	)

	if offline {
		config, err := ffproxy.NewFeatureFlagConfig(ffproxy.DefaultConfig, ffproxy.DefaultConfigDir)
		if err != nil {
			log.Fatal("failed to load config: ", err)
		}
		featureConfig = config.FeatureConfig()
		targetConfig = config.Targets()
	}

	memCache := cache.NewMemCache()
	tr, err := repository.NewTargetRepo(memCache, targetConfig)
	if err != nil {
		log.Fatal(err)
	}

	fcr, err := repository.NewFeatureConfigRepo(memCache, featureConfig)
	if err != nil {
		log.Fatal(err)
	}

	// We'll pass the above repos to the service once its been created but for
	// now just get a target and feature config and print them out

	ctx := context.Background()

	dave, err := tr.GetByIdentifier(ctx, domain.NewTargetKey("94ef7361-1f2d-40af-9b2c-c1145d537e5a"), "dave")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("target: ", dave)

	darkMode, err := fcr.GetByIdentifier(ctx, domain.NewFeatureConfigKey("94ef7361-1f2d-40af-9b2c-c1145d537e5a"), "harnessappdemodarkmode")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("featureConfig: ", darkMode)

}
