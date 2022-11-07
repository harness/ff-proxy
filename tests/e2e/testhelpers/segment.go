package testhelpers

import (
	"context"

	"github.com/harness/ff-proxy/gen/admin"
)

// CreateSegment ...
func CreateSegment(reqBody admin.CreateSegmentJSONRequestBody) (*admin.CreateSegmentResponse, error) {
	client := DefaultClient()

	segment, err := client.CreateSegment(context.Background(), &admin.CreateSegmentParams{
		AccountIdentifier: GetDefaultAccount(),
		OrgIdentifier:     GetDefaultOrg(),
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
