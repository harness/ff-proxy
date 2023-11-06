package testhelpers

import (
	"context"

	admin "github.com/harness/ff-proxy/v2/gen/admin"
)

// CreateSegment ...
func CreateSegment(org string, reqBody admin.CreateSegmentJSONRequestBody) (*admin.CreateSegmentResponse, error) {
	client := DefaultClient()

	segment, err := client.CreateSegment(context.Background(), &admin.CreateSegmentParams{
		AccountIdentifier: admin.AccountQueryParam(GetDefaultAccount()),
		OrgIdentifier:     admin.OrgQueryParam(org),
	}, reqBody, AddAuthToken)

	if err != nil {
		return nil, err
	}

	return admin.ParseCreateSegmentResponse(segment)
}

// GetSegmentRequestBody ...
func GetSegmentRequestBody(project string, environment string, segmentIdentifier string, segmentName string, included *[]string,
	excluded *[]string, tags *[]admin.Tag, clauses *[]admin.Clause) admin.CreateSegmentJSONRequestBody {
	return admin.CreateSegmentJSONRequestBody{
		Environment: environment,
		Excluded:    excluded,
		Identifier:  &segmentIdentifier,
		Included:    included,
		Name:        segmentName,
		Project:     project,
		Rules:       clauses,
		Tags:        tags,
	}
}
