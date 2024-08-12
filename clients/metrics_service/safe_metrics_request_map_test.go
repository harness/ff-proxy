package metricsservice

import (
	"sort"
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/stretchr/testify/assert"
)

func Test_SafeMetricsRequestMap(t *testing.T) {

	flagOneTrue := clientgen.MetricsData{
		Attributes: []clientgen.KeyValue{
			{
				Key:   "featureIdentifier",
				Value: "one",
			},
			{
				Key:   "variationIdentifier",
				Value: "true",
			},
			{
				Key:   "SDK_LANGUAGE",
				Value: "golang",
			},
			{
				Key:   "SDK_VERSION",
				Value: "1.0.0",
			},
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}
	flagOneTrueMetricsData := domain.ToPtr([]clientgen.MetricsData{flagOneTrue})

	makeMetricsRequest := func(envID string, size int, metricsData ...clientgen.MetricsData) domain.MetricsRequest {
		md := []clientgen.MetricsData{}
		md = append(md, metricsData...)

		return domain.MetricsRequest{
			Size:          size,
			EnvironmentID: envID,
			Metrics:       clientgen.Metrics{MetricsData: &md},
		}
	}

	flagOneFalse := clientgen.MetricsData{
		Attributes: []clientgen.KeyValue{
			{
				Key:   "featureIdentifier",
				Value: "one",
			},
			{
				Key:   "variationIdentifier",
				Value: "false",
			},
			{
				Key:   "SDK_LANGUAGE",
				Value: "golang",
			},
			{
				Key:   "SDK_VERSION",
				Value: "1.0.0",
			},
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}

	flagOneFalseGolangOne := clientgen.MetricsData{
		Attributes: []clientgen.KeyValue{
			{
				Key:   "featureIdentifier",
				Value: "one",
			},
			{
				Key:   "variationIdentifier",
				Value: "false",
			},
			{
				Key:   "SDK_LANGUAGE",
				Value: "golang",
			},
			{
				Key:   "SDK_VERSION",
				Value: "1.0.0",
			},
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}

	flagOneFalseGolangOneTwo := clientgen.MetricsData{
		Attributes: []clientgen.KeyValue{
			{
				Key:   "featureIdentifier",
				Value: "one",
			},
			{
				Key:   "variationIdentifier",
				Value: "false",
			},
			{
				Key:   "SDK_LANGUAGE",
				Value: "golang",
			},
			{
				Key:   "SDK_VERSION",
				Value: "1.2.0",
			},
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}

	flagOneFalseJaveOne := clientgen.MetricsData{
		Attributes: []clientgen.KeyValue{
			{
				Key:   "featureIdentifier",
				Value: "one",
			},
			{
				Key:   "variationIdentifier",
				Value: "false",
			},
			{
				Key:   "SDK_LANGUAGE",
				Value: "java",
			},
			{
				Key:   "SDK_VERSION",
				Value: "1.0.0",
			},
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}

	featureNameFlag := clientgen.MetricsData{
		Attributes: []clientgen.KeyValue{
			{
				Key:   "featureName",
				Value: "fname",
			},
			{
				Key:   "variationIdentifier",
				Value: "false",
			},
			{
				Key:   "SDK_LANGUAGE",
				Value: "java",
			},
			{
				Key:   "SDK_VERSION",
				Value: "1.0.0",
			},
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}

	featureNameFlag2 := clientgen.MetricsData{
		Attributes: []clientgen.KeyValue{
			{
				Key:   "featureName",
				Value: "fname2",
			},
			{
				Key:   "variationIdentifier",
				Value: "false",
			},
			{
				Key:   "SDK_LANGUAGE",
				Value: "java",
			},
			{
				Key:   "SDK_VERSION",
				Value: "1.0.0",
			},
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}

	featureNameAndIdentifierFlag := clientgen.MetricsData{
		Attributes: []clientgen.KeyValue{
			{
				Key:   "featureName",
				Value: "fname",
			},
			{
				Key:   "featureIdentifier",
				Value: "hello-world",
			},
			{
				Key:   "variationIdentifier",
				Value: "false",
			},
			{
				Key:   "SDK_LANGUAGE",
				Value: "java",
			},
			{
				Key:   "SDK_VERSION",
				Value: "1.0.0",
			},
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}

	type args struct {
		envID           string
		metricsRequests []domain.MetricsRequest
	}

	type expected struct {
		mapSize int
		data    map[string]domain.MetricsRequest
	}

	testCases := map[string]struct {
		args     args
		expected expected
	}{
		"Given I have an empty metrics request ": {
			args: args{
				envID:           "123",
				metricsRequests: []domain.MetricsRequest{},
			},
			expected: expected{
				data: map[string]domain.MetricsRequest{},
			},
		},
		"Given I have a metrics request with one flag and one variation": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, flagOneTrue),
				},
			},
			expected: expected{
				mapSize: 12,
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: flagOneTrueMetricsData,
						},
					},
				},
			},
		},
		"Given I have a two metrics requests for the same flag and variation in the same payload": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, flagOneTrue, flagOneTrue),
				},
			},
			expected: expected{
				mapSize: 12,
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								{
									Attributes:  flagOneTrue.Attributes,
									MetricsType: flagOneTrue.MetricsType,
									Timestamp:   flagOneTrue.Timestamp,

									// Expect count to be 2 because we've sent the same flag with a count of 1 twice
									Count: 2,
								},
							},
							),
						},
					},
				},
			},
		},
		"Given I have a two metrics requests for the same flag and variation in two different payloads": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, flagOneTrue),
					makeMetricsRequest("123", 12, flagOneTrue),
				},
			},
			expected: expected{
				mapSize: 12, // Expect size of 12 because the two flags should be aggregated into one object
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								{
									Attributes:  flagOneTrue.Attributes,
									MetricsType: flagOneTrue.MetricsType,
									Timestamp:   flagOneTrue.Timestamp,

									// Expect count to be 2 because we've sent the same flag with a count of 1 twice
									Count: 2,
								},
							},
							),
						},
					},
				},
			},
		},
		"Given I have a two metrics requests for the same flag but with different variations in the same payload": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, flagOneTrue, flagOneFalse),
				},
			},
			expected: expected{
				mapSize: 12,
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								flagOneTrue,
								flagOneFalse,
							}),
						},
					},
				},
			},
		},
		"Given I have a two metrics requests for the same flag but with different variations in two different payloads": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, flagOneTrue),
					makeMetricsRequest("123", 12, flagOneFalse),
				},
			},
			expected: expected{
				mapSize: 24, // Expect 24 because we've to store objects for both variations of the flag
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								flagOneTrue,
								flagOneFalse,
							}),
						},
					},
				},
			},
		},
		"Given I have a eight metrics requests for the two different flags with different variations": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, flagOneTrue),
					makeMetricsRequest("123", 12, flagOneFalse),
					makeMetricsRequest("123", 12, flagOneTrue),
					makeMetricsRequest("123", 12, flagOneFalse),
					makeMetricsRequest("123", 12, flagOneTrue),
					makeMetricsRequest("123", 12, flagOneFalse),
					makeMetricsRequest("123", 12, flagOneTrue),
					makeMetricsRequest("123", 12, flagOneFalse),
				},
			},
			expected: expected{
				mapSize: 24, // Expect 24 because we should have aggregated to two objects, one for each variation
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr(
								[]clientgen.MetricsData{
									{
										Attributes:  flagOneTrue.Attributes,
										MetricsType: flagOneTrue.MetricsType,
										Timestamp:   flagOneTrue.Timestamp,

										// Expect count to be 4 because we've sent the same flag & variation with a count of 4 twice
										Count: 4,
									},
									{
										Attributes:  flagOneFalse.Attributes,
										MetricsType: flagOneFalse.MetricsType,
										Timestamp:   flagOneFalse.Timestamp,

										// Expect count to be 4 because we've sent the same flag & variation with a count of 4 twice
										Count: 4,
									},
								},
							),
						},
					},
				},
			},
		},
		"Given I have a two metrics requests for the same flag, variation, sdkLanguage but different SDK versions": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, flagOneFalseGolangOne),
					makeMetricsRequest("123", 12, flagOneFalseGolangOneTwo),
				},
			},
			expected: expected{
				mapSize: 24, // Expect 24 because we've to store objects for both variations of the flag
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								flagOneFalseGolangOne,
								flagOneFalseGolangOneTwo,
							}),
						},
					},
				},
			},
		},
		"Given I have a two metrics requests for the same flag, variation but different sdk languages in different payloads": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, flagOneFalseGolangOne),
					makeMetricsRequest("123", 12, flagOneFalseJaveOne),
				},
			},
			expected: expected{
				mapSize: 24, // Expect 24 because we've to store objects for both variations of the flag
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								flagOneFalseGolangOne,
								flagOneFalseJaveOne,
							}),
						},
					},
				},
			},
		},
		"Given I have one metrics requests with only a featureName attribute": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, featureNameFlag),
				},
			},
			expected: expected{
				mapSize: 12,
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								featureNameFlag,
							}),
						},
					},
				},
			},
		},
		"Given I have two metrics requests with only a featureName attribute": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, featureNameFlag),
					makeMetricsRequest("123", 12, featureNameFlag2),
				},
			},
			expected: expected{
				mapSize: 24,
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								featureNameFlag,
								featureNameFlag2,
							}),
						},
					},
				},
			},
		},
		"Given I have one metrics requests with a featureName & featureIdentifier": {
			args: args{
				envID: "123",
				metricsRequests: []domain.MetricsRequest{
					makeMetricsRequest("123", 12, featureNameAndIdentifierFlag),
				},
			},
			expected: expected{
				mapSize: 12,
				data: map[string]domain.MetricsRequest{
					"123": {
						EnvironmentID: "123",
						Metrics: clientgen.Metrics{
							MetricsData: domain.ToPtr([]clientgen.MetricsData{
								featureNameAndIdentifierFlag,
							}),
						},
					},
				},
			},
		},
	}

	for desc, tc := range testCases {
		desc := desc
		tc := tc

		t.Run(desc, func(t *testing.T) {

			m2 := newSafeMetricsRequestMap()

			for _, mr := range tc.args.metricsRequests {
				m2.add(mr)
			}

			actual := m2.get()
			assert.Equal(t, len(tc.expected.data), len(actual))

			for k, v := range actual {
				expValue := tc.expected.data[k]

				expMetricData := domain.SafePtrDereference(expValue.MetricsData)

				sort.Slice(expMetricData, func(i, j int) bool {
					iKey := makeKey("", expMetricData[i].Attributes)
					jKey := makeKey("", expMetricData[j].Attributes)

					return iKey < jKey
				})

				actMetricData := domain.SafePtrDereference(v.MetricsData)
				sort.Slice(actMetricData, func(i, j int) bool {
					iKey := makeKey("", actMetricData[i].Attributes)
					jKey := makeKey("", actMetricData[j].Attributes)

					return iKey < jKey
				})

				assert.Equal(t, expMetricData, actMetricData)
			}

			assert.Equal(t, tc.expected.mapSize, m2.size())
		})
	}
}

func BenchmarkSafeMetricsRequestMapGetMethods(b *testing.B) {
	// Initialize your safeMetricsRequestMap with some data
	smr := newSafeMetricsRequestMap()

	// Populate smr with test data
	populateTestData(smr) // Assuming you have a function to populate test data

	// Table-driven benchmarks
	benchmarks := []struct {
		name string
		fn   func() map[string]domain.MetricsRequest
	}{
		{name: "get", fn: smr.get},
		{name: "getNew", fn: smr.getDeepCopyBenchmarkOnly},
	}

	for _, bm := range benchmarks {
		// Run the benchmark
		b.Run(bm.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = bm.fn() // Call the function and discard the result
			}
		})
	}
}

// populateTestData is a helper function to add test data to the map
func populateTestData(smr *safeMetricsRequestMap) {
	// Populate the map with sample data for benchmarking
	// This function should create a reasonably sized map to reflect a typical workload
	// Example:
	smr.add(domain.MetricsRequest{
		Size:          2,
		EnvironmentID: "env1",
		Metrics: clientgen.Metrics{
			MetricsData: domain.ToPtr([]clientgen.MetricsData{
				{Count: 3, Attributes: []clientgen.KeyValue{
					{
						Key:   "attr1",
						Value: "attr1",
					},
				}},
				{Count: 4, Attributes: []clientgen.KeyValue{
					{
						Key:   "attr2",
						Value: "attr2",
					},
				}},
			}),
		},
	})

	smr.add(domain.MetricsRequest{
		Size:          2,
		EnvironmentID: "env2",
		Metrics: clientgen.Metrics{
			MetricsData: domain.ToPtr([]clientgen.MetricsData{
				{Count: 3, Attributes: []clientgen.KeyValue{
					{
						Key:   "attr3",
						Value: "attr3",
					},
				}},
				{Count: 4, Attributes: []clientgen.KeyValue{
					{
						Key:   "attr4",
						Value: "attr4",
					},
				}},
			}),
		},
	})
	// Add more test data as needed
}

func (s *safeMetricsRequestMap) getDeepCopyBenchmarkOnly() map[string]domain.MetricsRequest {
	s.RLock()
	// Deep copy of the outer map
	cpy := make(map[string]map[string]clientgen.MetricsData, len(s.detailed))

	for envID, detailedMap := range s.detailed {
		// Deep copy of the nested map
		nestedCopy := make(map[string]clientgen.MetricsData, len(detailedMap))
		for key, value := range detailedMap {
			nestedCopy[key] = value
		}
		cpy[envID] = nestedCopy
	}
	s.RUnlock()

	result := make(map[string]domain.MetricsRequest, len(cpy))

	for envID, detailedMap := range cpy {
		slice := make([]clientgen.MetricsData, 0, len(detailedMap))
		for _, v := range detailedMap {
			slice = append(slice, v)
		}

		result[envID] = domain.MetricsRequest{
			EnvironmentID: envID,
			Metrics: clientgen.Metrics{
				MetricsData: domain.ToPtr(slice),
			},
		}
	}

	return result
}
