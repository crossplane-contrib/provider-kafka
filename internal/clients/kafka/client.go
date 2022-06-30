package kafka

import (
	"crypto/tls"
	"encoding/json"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// NewAdminClient creates a new AdminClient with supplied credentials
func NewAdminClient(data []byte) (*kadm.Client, error) {
	kc := Config{}

	if err := json.Unmarshal(data, &kc); err != nil {
		return nil, errors.Wrap(err, "cannot parse credentials")
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(kc.Brokers...),
		kgo.WithLogger(kgo.BasicLogger(os.Stdout, kgo.LogLevelWarn, nil)),
	}

	if kc.SASL != nil {
		var mechanism sasl.Mechanism
		switch name := kc.SASL.Mechanism; strings.ToLower(name) {
		case "plain":
			mechanism = plain.Auth{
				User: kc.SASL.Username,
				Pass: kc.SASL.Password,
			}.AsMechanism()
		case "scram-sha-512":
			mechanism = scram.Auth{
				User: kc.SASL.Username,
				Pass: kc.SASL.Password,
			}.AsSha512Mechanism()
		default:
			return nil, errors.Errorf("SASL mechanism %q not supported, only PLAIN / SCRAM-SHA-512 are supported for now.", kc.SASL.Mechanism)
		}
		opts = append(opts, kgo.SASL(mechanism))
	}

	if kc.TLS != nil {
		tc := new(tls.Config)
		opts = append(opts, kgo.DialTLSConfig(tc))
	}

	c, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return kadm.NewClient(c), nil
}
