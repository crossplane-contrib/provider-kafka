package topic

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kadm"

	"github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
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

const (
	errCannotListTopics           = "cannot list topics"
	errNoCreateResponse           = "no create response for topic"
	errCannotDescribeTopic        = "cannot describe topics"
	errCannotFindTopicInDescribe  = "cannot find topic in describe result"
	errErrorInTopicDescribeResult = "error in topic describe result"
	errNoCreateResponseForTopic   = "no create response for topic"
	errCannotCreateTopic          = "cannot create topic"
	errNoDeleteResponseForTopic   = "no delete response for topic"
	errCannotDeleteTopic          = "cannot delete topic"
	errCannotGetTopic             = "cannot get topic"
	errCannotUpdateTopicConfigs   = "cannot update topic configs"

	// ErrTopicDoesNotExist indicates that the topic of a given name doesn't exist in the external Kafka cluster
	ErrTopicDoesNotExist = "topic does not exist"
)

// Get gets the topic from Kafka side and returns a Topic object.
func Get(ctx context.Context, client *kadm.Client, name string) (*Topic, error) {
	td, err := client.ListTopics(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errCannotListTopics, err)
	}
	t := td[name]
	if t.Err != nil {
		return nil, fmt.Errorf("%s: %w", ErrTopicDoesNotExist, t.Err)
	}

	tc, err := client.DescribeTopicConfigs(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errCannotDescribeTopic, err)
	}

	ts := Topic{}
	ts.Name = name
	ts.Partitions = int32(len(t.Partitions))
	if len(t.Partitions) > 0 {
		ts.ReplicationFactor = int16(len(t.Partitions[0].Replicas))
	}
	ts.ID = t.ID.String()

	rc, err := tc.On(name, nil)
	if err != nil {
		return nil, fmt.Errorf(errCannotFindTopicInDescribe+": %w", err)
	}
	if rc.Err != nil {
		return nil, fmt.Errorf(errErrorInTopicDescribeResult+": %w", rc.Err)
	}
	ts.Config = make(map[string]*string, len(rc.Configs))
	for _, value := range rc.Configs {
		ts.Config[value.Key] = value.Value
	}
	return &ts, nil
}

// Create creates the topic from Kafka side. If the topic already exists, it
// returns nil (idempotent).
func Create(ctx context.Context, client *kadm.Client, topic *Topic) error {
	if _, err := Get(ctx, client, topic.Name); err == nil {
		return nil
	}

	resp, err := client.CreateTopics(ctx, topic.Partitions, topic.ReplicationFactor, topic.Config, topic.Name)
	if err != nil {
		return err
	}

	t, ok := resp[topic.Name]
	if !ok {
		return errors.New(errNoCreateResponseForTopic)
	}
	if t.Err != nil {
		return fmt.Errorf("%s: %w", errCannotCreateTopic, t.Err)
	}

	return nil
}

// Delete deletes the topic from Kafka side
func Delete(ctx context.Context, client *kadm.Client, name string) error {
	td, err := client.DeleteTopics(ctx, name)
	if err != nil {
		return err
	}

	t, ok := td[name]
	if !ok {
		return errors.New(errNoDeleteResponseForTopic)
	}
	if t.Err != nil {
		return fmt.Errorf("%s: %w", errCannotDeleteTopic, t.Err)
	}

	return nil
}

// Update determines if a Topic Partition or a Topic Admin Config update needs to be called and routes properly
func Update(ctx context.Context, client *kadm.Client, desired *Topic) error {
	existing, err := Get(ctx, client, desired.Name)
	if err != nil {
		return fmt.Errorf("%s: %w", errCannotGetTopic, err)
	}
	if existing == nil {
		return errors.New(ErrTopicDoesNotExist)
	}

	if desired.Partitions != existing.Partitions {
		return updatePartitions(ctx, client, desired, existing)
	}

	if desired.ReplicationFactor != existing.ReplicationFactor {
		return UpdateReplicationFactor()
	}

	if desired.Config != nil {
		return updateConfigs(ctx, client, desired, existing)
	}

	return nil
}

// updatePartitions updates a topic Partition count in Kafka, reusing the already-fetched existing topic.
func updatePartitions(ctx context.Context, client *kadm.Client, desired *Topic, existing *Topic) error {
	if desired.Partitions < existing.Partitions {
		return fmt.Errorf("cannot decrease topic partitions from %d to %d: Kafka does not support reducing the number of partitions",
			existing.Partitions, desired.Partitions)
	}
	resp, err := client.UpdatePartitions(ctx, int(desired.Partitions), desired.Name)
	if err != nil {
		return fmt.Errorf("cannot update topic partitions: %w", err)
	}
	r, err := resp.On(desired.Name, nil)
	if err != nil {
		return fmt.Errorf("cannot find topic in update partitions result: %w", err)
	}
	if r.Err != nil {
		return fmt.Errorf("error in update partitions result: %w", r.Err)
	}
	return nil
}

// UpdateReplicationFactor is not supported in Kafka. A user is given an error message
func UpdateReplicationFactor() error {
	return errors.New("updating replication factor is not supported")
}

// updateConfigs updates topic config keys that differ, batching all changes into a single Kafka call.
func updateConfigs(ctx context.Context, client *kadm.Client, desired *Topic, existing *Topic) error {
	var changes []kadm.AlterConfig
	for key, value := range desired.Config {
		if stringValue(value) != stringValue(existing.Config[key]) {
			changes = append(changes, kadm.AlterConfig{
				Op:    kadm.SetConfig,
				Name:  key,
				Value: value,
			})
		}
	}
	if len(changes) == 0 {
		return nil
	}
	r, err := client.AlterTopicConfigs(ctx, changes, desired.Name)
	if err != nil {
		return fmt.Errorf("%s: %w", errCannotUpdateTopicConfigs, err)
	}
	if len(r) > 0 && r[0].Err != nil {
		return fmt.Errorf("%s: %w", errCannotUpdateTopicConfigs, r[0].Err)
	}
	return nil
}

// Generate is used to convert Crossplane TopicParameters to Kafka's Topic.
func Generate(name string, params *v1alpha1.TopicParameters) *Topic {
	tpc := &Topic{
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

// IsUpToDate returns true if the supplied Kubernetes resource matches the
// supplied Kafka Topic. Spec config keys not present in observed or with
// different values trigger an update. Broker defaults not in spec are ignored.
func IsUpToDate(in *v1alpha1.TopicParameters, observed *Topic) bool {
	if in.Partitions != int(observed.Partitions) {
		return false
	}
	if in.ReplicationFactor != int(observed.ReplicationFactor) {
		return false
	}
	for k, v := range in.Config {
		observedConfigValue, ok := observed.Config[k]
		if !ok || stringValue(v) != stringValue(observedConfigValue) {
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
