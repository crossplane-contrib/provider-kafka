package kafka

import (
	"encoding/json"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/pkg/errors"
)

// NewAdminClient creates a new AdminClient with supplied credentials
func NewAdminClient(data []byte) (*kafka.AdminClient, error) {
	conf := &kafka.ConfigMap{}
	if err := json.Unmarshal(data, conf); err != nil {
		return nil, errors.Wrap(err, "cannot parse credentials")
	}

	return kafka.NewAdminClient(conf)
}
