package kafka

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
)

func NewAdminClient(data []byte) (*kadm.Client, error) {
	kc := Config{}

	if err := json.Unmarshal(data, &kc); err != nil {
		return nil, errors.Wrap(err, "cannot parse credentials")
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(kc.Brokers...),
	}

	if kc.SASL != nil {
		opts = append(opts, kgo.SASL(plain.Auth{
			User: kc.SASL.Username,
			Pass: kc.SASL.Password,
		}.AsMechanism()))
	}

	c, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return kadm.NewClient(c), nil
}
