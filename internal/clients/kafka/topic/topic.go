package topic

import (
	"context"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"

	"fmt"

	"github.com/crossplane-contrib/provider-kafka/apis/topic/v1alpha1"
)

// Topic is a holistic representation of a Kafka Topic with all configurable
// fields
//type TopicParameters struct {
//	Topic             string
//	NumPartitions     int
//	ReplicationFactor int
//	ReplicaAssignment [][]int32
//	Config            map[string]string
//}

// Get gets the topic from Kafka side and returns a Topic object.
func Get(ctx context.Context, client *kafka.AdminClient, name string) (*kafka.TopicSpecification, error) {

	tpc, err := client.GetMetadata(&name, true, 1000)
	if err != nil {
		return nil, errors.Wrap(err, "cannot get the topic")
	}
	t := tpc.Topics
	fmt.Println(t)
	return nil, nil

}

// Create creates the topic from Kafka side
func Create(ctx context.Context, client *kafka.AdminClient, topic *kafka.TopicSpecification) error {
	tsp := kafka.TopicSpecification{
		Topic:             topic.Topic,
		NumPartitions:     topic.NumPartitions,
		ReplicationFactor: topic.ReplicationFactor,
	}
	ts := make([]kafka.TopicSpecification, 1)
	ts[0] = tsp
	//_ = append(ts, topicD[0])
	_, err := client.CreateTopics(ctx, ts)
	if err != nil {
		return errors.Wrap(err, "cannot create topic")
	}
	return nil
}

// Delete deletes the topic from Kafka side
func Delete(ctx context.Context, client *kafka.AdminClient, name string) error {
	ts := []string{name}

	_, err := client.DeleteTopics(ctx, ts)
	if err != nil {
		return errors.Wrap(err, "cannot delete topic")
	}
	return nil
}

// Update determines if a Topic Partition or a Topic Admin Config update needs to be called and routes properly
func Update(ctx context.Context, client *kafka.AdminClient, desired *kafka.TopicSpecification) error {
	// First Get existing Topic
	// Check that the partitions are equal desired/existing
	// Check that config is not nil

	return nil
}

// UpdatePartitions updates a topic Partition count in Kafka
func UpdatePartitions(ctx context.Context, client *kadm.Client, desired *kafka.TopicSpecification) error {
	// First Get existing Topic
	// Check if partitions are not equal from existing/desired
	// Update the partitions based on existing/desired

	// Replication factor is not supported for update

	return nil
}

// UpdateConfigs updates an optional topic Admin Configuration in Kafka
func UpdateConfigs(ctx context.Context, client *kadm.Client, desired *kafka.TopicSpecification) error {
	// First Get existing Topic
	// Check that desired config is not nil
	// Parse config range into a useful config struct to be passed
	// Use alter topic configs to update the topic configs

	return nil
}

// Generate is used to convert Crossplane TopicParameters to Kafka's Topic.
func Generate(name string, params *v1alpha1.TopicParameters) *kafka.TopicSpecification {
	// Use given configurations to build a struct that is usable when updating a topic
	tpc := &kafka.TopicSpecification{
		Topic:             name,
		ReplicationFactor: params.ReplicationFactor,
		NumPartitions:     params.Partitions,
		Config:            params.Config,
	}
	return tpc
}

// LateInitializeSpec fills empty spec fields with the data retrieved from Kafka.
func LateInitializeSpec(params *v1alpha1.TopicParameters, observed *kafka.TopicSpecification) bool {
	lateInitialized := false
	// Modify to use new struct for Observed
	if params.Config == nil {
		params.Config = make(map[string]string, len(observed.Config))
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
func IsUpToDate(in *v1alpha1.TopicParameters, observed *kafka.TopicSpecification) bool {
	// Modify to use new struct for Observed
	if in.Partitions != int(observed.NumPartitions) {
		return false
	}
	if in.ReplicationFactor != int(observed.ReplicationFactor) {
		return false
	}
	if len(in.Config) != len(observed.Config) {
		return false
	}
	for k, v := range observed.Config {
		if iv, ok := in.Config[k]; !ok || stringValue(&iv) != stringValue(&v) {
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
