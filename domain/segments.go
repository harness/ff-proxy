package domain

import (
	"fmt"

	"github.com/harness/ff-golang-server-sdk/rest"
	clientgen "github.com/harness/ff-proxy/v2/gen/client"
	jsoniter "github.com/json-iterator/go"
)

// SegmentKey is the key that maps to a Segment
type SegmentKey string

// NewSegmentKey creates a SegmentKey from an environment and identifier
func NewSegmentKey(envID string, identifier string) SegmentKey {
	return SegmentKey(fmt.Sprintf("env-%s-segment-%s", envID, identifier))
}

// NewSegmentsKey creates a SegmentKey from an environment
func NewSegmentsKey(envID string) SegmentKey {
	return SegmentKey(fmt.Sprintf("env-%s-segments", envID))
}

// Segment is a rest.Segment that we can declare methods on
type Segment clientgen.Segment

// MarshalBinary marshals a Segment to bytes. Currently it uses json marshaling
// but if we want to optimise storage space we could use something more efficient
func (s *Segment) MarshalBinary() ([]byte, error) {
	return jsoniter.Marshal(s)
}

// UnmarshalBinary unmarshals bytes to a Segment
func (s *Segment) UnmarshalBinary(b []byte) error {
	return jsoniter.Unmarshal(b, s)
}

func (s *Segment) ToSDKSegment() rest.Segment {

	rules := []rest.Clause{}
	if *s.Rules != nil {
		rules = toSDKClause(*s.Rules)
	}

	excluded := toSDKTarget(s.Excluded)
	included := toSDKTarget(s.Included)

	return rest.Segment{
		CreatedAt:   s.CreatedAt,
		Environment: s.Environment,
		Excluded:    &excluded,
		Identifier:  s.Identifier,
		Included:    &included,
		ModifiedAt:  s.ModifiedAt,
		Name:        s.Name,
		Rules:       &rules,
		Tags:        nil,
		Version:     s.Version,
	}
}

func toSDKTarget(targets *[]clientgen.Target) []rest.Target {
	if targets == nil {
		return []rest.Target{}
	}

	result := make([]rest.Target, 0, len(*targets))
	for _, t := range *targets {
		result = append(result, rest.Target{
			Account:     t.Account,
			Anonymous:   t.Anonymous,
			Attributes:  t.Attributes,
			CreatedAt:   t.CreatedAt,
			Environment: t.Environment,
			Identifier:  t.Identifier,
			Name:        t.Name,
			Org:         t.Org,
			Project:     t.Project,
			Segments:    nil,
		})
	}

	return result
}
