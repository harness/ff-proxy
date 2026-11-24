package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/redis/go-redis/v9"
)

// self-contained so this file can be run against any revision of hash_cache.go
func newBenchBackend(b *testing.B) *KeyValCache {
	b.Helper()
	server := miniredis.RunT(b)
	client := redis.NewClient(&redis.Options{Addr: server.Addr(), MaxRetries: -1})
	b.Cleanup(func() { _ = client.Close() })
	return NewKeyValCache(client)
}

func benchFlags(n int) []domain.FeatureFlag {
	flags := make([]domain.FeatureFlag, 0, n)
	for i := 0; i < n; i++ {
		flags = append(flags, domain.FeatureFlag{
			Feature:      fmt.Sprintf("flag-%d", i),
			Project:      "bench-project",
			Environment:  "bench-env",
			OffVariation: "false",
			Variations: []clientgen.Variation{
				{Identifier: "true", Value: "true"},
				{Identifier: "false", Value: "false"},
			},
		})
	}
	return flags
}

// BenchmarkHashCacheGetHit measures the latest-only read path when the local
// snapshot is current, which is the steady state for SDK polling.
func BenchmarkHashCacheGetHit(b *testing.B) {
	for _, size := range []int{50, 500} {
		b.Run(fmt.Sprintf("flags=%d", size), func(b *testing.B) {
			ctx := context.Background()
			hc := NewHashCache(newBenchBackend(b), time.Minute, 0, WithLatestOnly(true))
			key := string(domain.NewFeatureConfigsKey("bench"))
			if err := hc.Set(ctx, key, benchFlags(size)); err != nil {
				b.Fatal(err)
			}

			var got []domain.FeatureFlag
			if err := hc.Get(ctx, key, &got); err != nil {
				b.Fatal(err)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := hc.Get(ctx, key, &got); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkHashCacheSet measures the write-through path the Refresher takes on
// every SSE-driven flag change.
func BenchmarkHashCacheSet(b *testing.B) {
	ctx := context.Background()
	hc := NewHashCache(newBenchBackend(b), time.Minute, 0, WithLatestOnly(true))
	key := string(domain.NewFeatureConfigsKey("bench"))
	flags := benchFlags(500)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := hc.Set(ctx, key, flags); err != nil {
			b.Fatal(err)
		}
	}
}
