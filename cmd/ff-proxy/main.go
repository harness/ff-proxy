package main

import (
	"flag"
	"fmt"
	"log"

	ffproxy "github.com/harness/ff-proxy"
	"github.com/harness/ff-proxy/gen"
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
		featureConfig map[ffproxy.FeatureConfigKey][]*ffproxy.FeatureConfig
		targetConfig  map[ffproxy.TargetKey][]*gen.Target
	)

	if offline {
		config, err := ffproxy.NewFeatureFlagConfig(ffproxy.DefaultConfig, ffproxy.DefaultConfigDir)
		if err != nil {
			log.Fatal("failed to load config: ", err)
		}
		featureConfig = config.FeatureConfig()
		targetConfig = config.Targets()
	}

	// Just print these for now, once we've implemented our cache we can pass
	// these to it
	fmt.Println(featureConfig)
	fmt.Println(targetConfig)
}
