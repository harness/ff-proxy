package proxyservice

import "github.com/harness/ff-golang-server-sdk/rest"

type QueryStore struct {
	F func(identifier string) (rest.FeatureConfig, error)
	S func(identifier string) (rest.Segment, error)
	L func() ([]rest.FeatureConfig, error)
}

func (q QueryStore) GetFlag(identifier string) (rest.FeatureConfig, error) {
	return q.F(identifier)
}

func (q QueryStore) GetSegment(identifier string) (rest.Segment, error) {
	return q.S(identifier)
}
func (q QueryStore) GetFlags() ([]rest.FeatureConfig, error) {
	return q.L()
}
