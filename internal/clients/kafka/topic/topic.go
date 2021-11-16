package topic

import (
	"context"

	"github.com/pkg/errors"
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
	Config            map[string]string
}

// Get gets the topic from Kafka side and returns a Topic object.
func Get(ctx context.Context, client *kadm.Client, name string) (*Topic, error) {
	// TODO: We first need to get the topic (via ListTopics) to fill the ID,
	//  ReplicationFactor and Partitions fields. Then call DescribeTopicConfigs
	//  to fill Topic.Config.
	return &Topic{}, nil
}

func Create(ctx context.Context, client *kadm.Client, topic *Topic) error {
	// TODO: Call client.CreateTopics using provided Topic

	// TODO: Could we pass topic configs in Create call? Otherwise, also call
	//  AlterConfig for each config provided.
	return nil
}

// Delete deletes the topic from Kafka side
func Delete(ctx context.Context, client *kadm.Client, name string) error {
	// TODO: Call client.DeleteTopics
	return nil
}

func Update(ctx context.Context, client *kadm.Client, desired *Topic) error {
	// First Get existing Topic
	existing, err := Get(ctx, client, desired.Name)
	if err != nil {
		return errors.Wrap(err, "cannot get topic")
	}
	if existing == nil {
		return errors.New("topic does not exist")
	}
	// TODO: Update if Partitions needs to be updated

	// TODO: Update all configs as in the spec. Yes, we might call an Update
	//  (i.e. Set), also for the ones didn't change, but this shouldn't be
	//  a problem, given we do that only when something is not up to date.
	return nil
}

// Generate is used to convert Crossplane TopicParameters to Kafka's Topic.
func Generate(name string, params *v1alpha1.TopicParameters) *Topic {
	tpc := &Topic{
		Name:              name,
		ReplicationFactor: int16(params.ReplicationFactor),
		Partitions:        int32(params.Partitions),
	}
	tpc.Config = make(map[string]string, len(params.Config))
	for k, v := range params.Config {
		tpc.Config[k] = v
	}

	return tpc
}

// LateInitializeSpec fills empty spec fields with the data retrieved from Kafka.
func LateInitializeSpec(params *v1alpha1.TopicParameters, observed *Topic) bool {
	lateInitialized := false
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
func IsUpToDate(in *v1alpha1.TopicParameters, observed *Topic) bool {
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
		if iv, ok := in.Config[k]; !ok || iv != v {
			return false
		}
	}
	return true
}
