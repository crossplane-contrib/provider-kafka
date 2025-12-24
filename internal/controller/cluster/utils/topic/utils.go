package topic

import (
	"github.com/crossplane-contrib/provider-kafka/apis/cluster/topic/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/topic"
)

// Generate is used to convert Crossplane TopicParameters to Kafka's Topic.
func Generate(name string, params *v1alpha1.TopicParameters) *topic.Topic {
	tpc := &topic.Topic{
		Name:              name,
		ReplicationFactor: int16(params.ReplicationFactor),
		Partitions:        int32(params.Partitions),
	}

	if len(params.Config) > 0 {
		tpc.Config = make(map[string]*string, len(params.Config))
		for k, v := range params.Config {
			tpc.Config[k] = v
		}
	}

	return tpc
}

// LateInitializeSpec fills empty spec fields with the data retrieved from Kafka.
func LateInitializeSpec(params *v1alpha1.TopicParameters, observed *topic.Topic) bool {
	lateInitialized := false
	if params.Config == nil {
		params.Config = make(map[string]*string, len(observed.Config))
	}

	for k, v := range observed.Config {
		if _, ok := params.Config[k]; !ok {
			lateInitialized = true
			params.Config[k] = v
		}
	}
	return lateInitialized
}

// IsUpToDate returns true if the supplied Kubernetes resource differs from the
// supplied Kafka Topic.
func IsUpToDate(in *v1alpha1.TopicParameters, observed *topic.Topic) bool {
	if in.Partitions != int(observed.Partitions) {
		return false
	}
	if in.ReplicationFactor != int(observed.ReplicationFactor) {
		return false
	}
	if len(in.Config) != len(observed.Config) {
		return false
	}
	for k, v := range observed.Config {
		if iv, ok := in.Config[k]; !ok || stringValue(iv) != stringValue(v) {
			return false
		}
	}
	return true
}

func stringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
