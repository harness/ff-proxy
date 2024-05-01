package metricsservice

import (
	"testing"

	"github.com/harness/ff-proxy/v2/domain"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	"github.com/stretchr/testify/assert"
)

func TestMap2_Add(t *testing.T) {

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
		},
		Count:       1,
		MetricsType: "Server",
		Timestamp:   0,
	}

	//flagTwoFalse := clientgen.MetricsData{
	//	Attributes: []clientgen.KeyValue{
	//		{
	//			Key:   "featureIdentifier",
	//			Value: "two",
	//		},
	//		{
	//			Key:   "variationIdentifier",
	//			Value: "false",
	//		},
	//	},
	//	Count:       1,
	//	MetricsType: "Server",
	//	Timestamp:   0,
	//}
	//
	//flagTwoTrue := clientgen.MetricsData{
	//	Attributes: []clientgen.KeyValue{
	//		{
	//			Key:   "featureIdentifier",
	//			Value: "two",
	//		},
	//		{
	//			Key:   "variationIdentifier",
	//			Value: "true",
	//		},
	//	},
	//	Count:       1,
	//	MetricsType: "Server",
	//	Timestamp:   0,
	//}

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

				assert.Equal(t, expValue, v)
			}

			assert.Equal(t, tc.expected.mapSize, m2.size())
		})
	}
}
