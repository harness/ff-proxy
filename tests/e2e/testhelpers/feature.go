package testhelpers

import (
	"context"
	"fmt"

	"github.com/harness/ff-proxy/gen/admin"
)

// GenerateBooleanFeatureFlagBody ...
func GenerateBooleanFeatureFlagBody(projectIdentifier string, id int) admin.CreateFeatureFlagJSONRequestBody {
	identifier := fmt.Sprintf("bool-flag%d", id)
	description := fmt.Sprintf("Basic boolean flag %d", id)
	trueVariationName := trueVariationName
	trueVariationDesc := trueVariationDescription
	falseVariationName := falseVariationName
	falseVariationDesc := falseVariationDescription

	return admin.CreateFeatureFlagJSONRequestBody{
		DefaultOffVariation: "true",
		DefaultOnVariation:  "true",
		Description:         &description,
		Identifier:          identifier,
		Kind:                "boolean",
		Name:                fmt.Sprintf("Basic bool flag %d", id),
		Owner:               nil,
		Permanent:           false,
		Project:             projectIdentifier,
		Tags:                nil,
		Variations: []admin.Variation{
			{
				Identifier:  trueVariationIdentifier,
				Value:       trueVariationValue,
				Name:        &trueVariationName,
				Description: &trueVariationDesc,
			},
			{
				Identifier:  falseVariationIdentifier,
				Value:       falseVariationValue,
				Name:        &falseVariationName,
				Description: &falseVariationDesc,
			},
		},
	}
}

// GenerateStringFeatureFlagBody ...
func GenerateStringFeatureFlagBody(projectIdentifier string, id int) admin.CreateFeatureFlagJSONRequestBody {
	identifier := fmt.Sprintf("string-flag%d", id)
	description := fmt.Sprintf("Basic string flag %d", id)
	variation1 := redVariationName
	variation1Desc := redVariationDescription
	variation2 := blueVariationName
	variation2Desc := blueVariationDescription
	return admin.CreateFeatureFlagJSONRequestBody{
		DefaultOffVariation: redVariationIdentifier,
		DefaultOnVariation:  blueVariationIdentifier,
		Description:         &description,
		Identifier:          identifier,
		Kind:                "string",
		Name:                fmt.Sprintf("Basic string flag %d", id),
		Owner:               nil,
		Permanent:           false,
		Project:             projectIdentifier,
		Tags:                nil,
		Variations: []admin.Variation{
			{
				Identifier:  redVariationIdentifier,
				Value:       redVariationValue,
				Name:        &variation1,
				Description: &variation1Desc,
			},
			{
				Identifier:  blueVariationIdentifier,
				Value:       blueVariationValue,
				Name:        &variation2,
				Description: &variation2Desc,
			},
		},
	}
}

// CreateFeatureFlag ...
func CreateFeatureFlag(reqBody admin.CreateFeatureFlagJSONRequestBody) (*admin.CreateFeatureFlagResponse, error) {
	client := DefaultClient()

	flag, err := client.CreateFeatureFlag(context.Background(), &admin.CreateFeatureFlagParams{
		AccountIdentifier: admin.AccountQueryParam(GetDefaultAccount()),
		OrgIdentifier:     admin.OrgQueryParam(GetDefaultOrg()),
	}, reqBody, AddAuthToken)

	if err != nil {
		return nil, err
	}

	return admin.ParseCreateFeatureFlagResponse(flag)
}
