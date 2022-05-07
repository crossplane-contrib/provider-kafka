package kafka

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	kaws "github.com/twmb/franz-go/pkg/sasl/aws"
	"github.com/twmb/franz-go/pkg/sasl/plain"
)

// NewAdminClient creates a new AdminClient with supplied credentials
func NewAdminClient(data []byte) (*kadm.Client, error) {
	kc := Config{}

	if err := json.Unmarshal(data, &kc); err != nil {
		return nil, errors.Wrap(err, "cannot parse credentials")
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(kc.Brokers...),
	}

	if kc.SASL != nil {
		s, err := authenticateWithSASL(kc.SASL)
		if err != nil {
			return nil, errors.Wrap(err, "unable to perform authentication")
		}
		opts = append(opts, kgo.SASL(s))
	}

	c, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return kadm.NewClient(c), nil
}

func authenticateWithSASL(sasl *SASL) (sasl.Mechanism, error) {
	switch sasl.Mechanism {
	case "PLAIN":
		return plain.Auth{
			User: sasl.Username,
			Pass: sasl.Password,
		}.AsMechanism(), nil
	case "AWS_MSK_IAM":
		return awsMskIamAuthentication(sasl)
	default:
		return nil, errors.Errorf("SASL mechanisms %q not supported, only %q supported for now.", sasl.Mechanism, "PLAIN & AWS_MSK_IAM")
	}
}

func awsMskIamAuthentication(sasl *SASL) (sasl.Mechanism, error) {
	s, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	if sasl.RoleArn != "" {
		return assumeAwsRole(s, sasl.RoleArn)
	}

	val, err := s.Config.Credentials.GetWithContext(context.TODO())
	if err != nil {
		return nil, err
	}

	a := kaws.Auth{
		AccessKey:    val.AccessKeyID,
		SecretKey:    val.SecretAccessKey,
		SessionToken: val.SessionToken,
	}

	return a.AsManagedStreamingIAMMechanism(), nil
}

func assumeAwsRole(s *session.Session, roleArn string) (sasl.Mechanism, error) {
	sc := sts.New(s)
	sn := "crossplane_provider_kafka_session"

	res, err := sc.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         &roleArn,
		RoleSessionName: &sn,
	})
	if err != nil {
		return nil, err
	}

	a := kaws.Auth{
		AccessKey:    *res.Credentials.AccessKeyId,
		SecretKey:    *res.Credentials.SecretAccessKey,
		SessionToken: *res.Credentials.SessionToken,
	}

	return a.AsManagedStreamingIAMMechanism(), nil
}
