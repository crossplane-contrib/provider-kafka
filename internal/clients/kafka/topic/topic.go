package topic

import (
	"context"

	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
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
		return nil, errors.Wrap(err, errCannotListTopics)
	}
	if td[name].Err != nil {
		return nil, errors.Wrap(td[name].Err, ErrTopicDoesNotExist)
	}

	t, ok := td[name]
	if !ok {
		return nil, errors.New(errNoCreateResponse)
	}

	tc, err := client.DescribeTopicConfigs(ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, errCannotDescribeTopic)
	}

	ts := Topic{}
	ts.Name = name
	ts.Partitions = int32(len(t.Partitions))
	if len(t.Partitions) > 0 {
		ts.ReplicationFactor = int16(len(t.Partitions[0].Replicas))
	}
	ts.ID = t.ID.String()
	ts.Config = make(map[string]*string, len(ts.Config))

	rc, err := tc.On(name, nil)
	if err != nil {
		return nil, errors.Wrapf(err, errCannotFindTopicInDescribe)
	}
	if rc.Err != nil {
		return nil, errors.Wrapf(rc.Err, errErrorInTopicDescribeResult)
	}
	for _, value := range rc.Configs {
		ts.Config[value.Key] = value.Value
	}
	return &ts, nil

}

// Create creates the topic from Kafka side
func Create(ctx context.Context, client *kadm.Client, topic *Topic) error {
	resp, err := client.CreateTopics(ctx, topic.Partitions, topic.ReplicationFactor, topic.Config, topic.Name)
	if err != nil {
		return err
	}

	t, ok := resp[topic.Name]
	if !ok {
		return errors.New(errNoCreateResponseForTopic)
	}
	if t.Err != nil {
		return errors.Wrap(t.Err, errCannotCreateTopic)
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
		return errors.Wrap(t.Err, errCannotDeleteTopic)
	}

	return nil
}

// Update determines if a Topic Partition or a Topic Admin Config update needs to be called and routes properly
func Update(ctx context.Context, client *kadm.Client, desired *Topic) error {
	existing, err := Get(ctx, client, desired.Name)
	if err != nil {
		return errors.Wrap(err, errCannotGetTopic)
	}
	if existing == nil {
		return errors.New(ErrTopicDoesNotExist)
	}

	if desired.Partitions != existing.Partitions {
		return UpdatePartitions(ctx, client, desired)
	}

	if desired.ReplicationFactor != existing.ReplicationFactor {
		return UpdateReplicationFactor()
	}

	if desired.Config != nil {
		return UpdateConfigs(ctx, client, desired)
	}

	return nil
}

// UpdatePartitions updates a topic Partition count in Kafka
func UpdatePartitions(ctx context.Context, client *kadm.Client, desired *Topic) error {
	existing, err := Get(ctx, client, desired.Name)
	if err != nil {
		return errors.Wrap(err, errCannotGetTopic)
	}
	if existing == nil {
		return errors.New(ErrTopicDoesNotExist)
	}

	if desired.Partitions != existing.Partitions {
		resp, err := client.UpdatePartitions(ctx, int(desired.Partitions), desired.Name)
		if err != nil {
			return errors.Wrap(err, "cannot update topic partitions")
		}
		r, err := resp.On(desired.Name, nil)
		if err != nil {
			return errors.Wrap(err, "cannot find topic in update partitions result")
		}
		if r.Err != nil {
			return errors.Wrap(r.Err, "error in update partitions result")
		}
	}

	return nil
}

// UpdateReplicationFactor is not supported in Kafka. A user is given an error message
func UpdateReplicationFactor() error {

	return errors.New("updating replication factor is not supported")
}

// UpdateConfigs updates an optional topic Admin Configuration in Kafka
func UpdateConfigs(ctx context.Context, client *kadm.Client, desired *Topic) error {
	existing, err := Get(ctx, client, desired.Name)
	if err != nil {
		return errors.Wrap(err, errCannotGetTopic)
	}
	if existing == nil {
		return errors.New("topic does not exist")
	}

	if desired.Config != nil {
		configs := desired.Config
		existing := existing.Config

		for key, value := range configs {
			ev := existing[key]
			if stringValue(value) != stringValue(ev) {
				s := kadm.AlterConfig{
					Op:    kadm.SetConfig, // Op is the incremental alter operation to perform.
					Name:  key,            // Name is the name of the config to alter.
					Value: value,          // Value is the value to use when altering, if any.
				}
				r, err := client.AlterTopicConfigs(ctx, []kadm.AlterConfig{s}, desired.Name)
				if err != nil {
					return errors.Wrap(err, errCannotUpdateTopicConfigs)
				}
				if r[0].Err != nil {
					return errors.Wrap(r[0].Err, errCannotUpdateTopicConfigs)
				}
			}
		}
	}

	return nil
}

func stringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
