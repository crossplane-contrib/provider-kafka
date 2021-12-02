package topic

import (
	"context"
	"github.com/confluentinc/confluent-kafka-go/kafka"

	"github.com/twmb/franz-go/pkg/kadm"

	"github.com/crossplane-contrib/provider-kafka/apis/topic/v1alpha1"
)

// Topic is a holistic representation of a Kafka Topic with all configurable
// fields
type Topic struct {
	Name              string
	ReplicationFactor int16
	Partitions        int32
	ID                string
	Config            map[string]*string
}

// Get gets the topic from Kafka side and returns a Topic object.
func Get(ctx context.Context, client *kafka.AdminClient, name string) (*Topic, error) {

	return nil, nil

}

// Create creates the topic from Kafka side
func Create(ctx context.Context, client *kafka.AdminClient, topic *Topic) error {

	return nil
}

// Delete deletes the topic from Kafka side
func Delete(ctx context.Context, client *kafka.AdminClient, name string) error {

	return nil
}

// Update determines if a Topic Partition or a Topic Admin Config update needs to be called and routes properly
func Update(ctx context.Context, client *kafka.AdminClient, desired *Topic) error {
	// First Get existing Topic
	// Check that the partitions are equal desired/existing
	// Check that config is not nil

	return nil
}

// UpdatePartitions updates a topic Partition count in Kafka
func UpdatePartitions(ctx context.Context, client *kadm.Client, desired *Topic) error {
	// First Get existing Topic
	// Check if partitions are not equal from existing/desired
	// Update the partitions based on existing/desired

	// Replication factor is not supported for update

	return nil
}

// UpdateConfigs updates an optional topic Admin Configuration in Kafka
func UpdateConfigs(ctx context.Context, client *kadm.Client, desired *Topic) error {
	// First Get existing Topic
	// Check that desired config is not nil
	// Parse config range into a useful config struct to be passed
	// Use alter topic configs to update the topic configs

	return nil
}

// Generate is used to convert Crossplane TopicParameters to Kafka's Topic.
func Generate(name string, params *v1alpha1.TopicParameters) *Topic {
	// Use given configurations to build a struct that is useable when updating a topic

	return nil
}

// LateInitializeSpec fills empty spec fields with the data retrieved from Kafka.
func LateInitializeSpec(params *v1alpha1.TopicParameters, observed *Topic) bool {
	lateInitialized := false
	// Modify to use new struct for Observed
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
func IsUpToDate(in *v1alpha1.TopicParameters, observed *Topic) bool {
	// Modify to use new struct for Observed
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
