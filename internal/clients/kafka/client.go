package kafka

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func NewAdminClient(creds []byte) (*kafka.AdminClient, error) {
	// Create the new admin client using k8s secrets
	// For now, client looks like:

	// TODO: Remove hard coding!  Needs to use config.go
	return kafka.NewAdminClient(&kafka.ConfigMap{
		"bootstrap.servers": "kafka-dev-0.kafka-dev-headless:9092",
		"sasl.username":     "user",
		"sasl.password":     "PASSWORD",
		"sasl.mechanism":    "PLAIN",
		"security.protocol": "SASL_PLAINTEXT",
	})
}
