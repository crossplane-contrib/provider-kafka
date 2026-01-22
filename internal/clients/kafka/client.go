package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	kaws "github.com/twmb/franz-go/pkg/sasl/aws"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// default Secret field names for TLS certificates, like managed by cert-manager
	defaultClientCertificateKeyField  = "tls.key"
	defaultClientCertificateCertField = "tls.crt"

	errCannotParse                    = "cannot parse credentials"
	errMissingSASLMechanism           = "SASL mechanism is required"
	errMissingSASLCredentials         = "SASL username and password are required"
	errMissingClientCertSecretRefKeys = "missing client cert ref secret name or namespace"
	errCannotReadClientCertSecret     = "cannot read client cert secret"
)

// NewAdminClient creates a new AdminClient with supplied credentials
func NewAdminClient(ctx context.Context, data []byte, kube client.Client) (*kadm.Client, error) { // nolint: gocyclo
	kc := Config{}

	if err := json.Unmarshal(data, &kc); err != nil {
		return nil, errors.Wrap(err, errCannotParse)
	}

	// Validate SASL configuration if provided
	if kc.SASL != nil {
		if kc.SASL.Mechanism == "" {
			return nil, errors.New(errMissingSASLMechanism)
		}
		// AWS MSK IAM uses IAM credentials, not username/password
		if !strings.EqualFold(kc.SASL.Mechanism, "aws-msk-iam") {
			if kc.SASL.Username == "" || kc.SASL.Password == "" {
				return nil, errors.New(errMissingSASLCredentials)
			}
		}
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
		case "aws-msk-iam":
			mechanism = kaws.ManagedStreamingIAM(authenticateAwsIam)
			opts = append(opts, kgo.Dialer((&tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}}).DialContext))
		case "scram-sha-512":
			mechanism = scram.Auth{
				User: kc.SASL.Username,
				Pass: kc.SASL.Password,
			}.AsSha512Mechanism()
		default:
			return nil, errors.Errorf("SASL mechanism %q not supported, only PLAIN / SCRAM-SHA-512 / AWS-MSK-IAM are supported for now.", kc.SASL.Mechanism)
		}
		opts = append(opts, kgo.SASL(mechanism))
	}

	if kc.TLS != nil {
		tc := new(tls.Config)
		tc.InsecureSkipVerify = kc.TLS.InsecureSkipVerify
		if err := configureClientCertificate(ctx, kc, kube, tc); err != nil {
			return nil, err
		}
		opts = append(opts, kgo.DialTLSConfig(tc))
	}

	c, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return kadm.NewClient(c), nil
}

func authenticateAwsIam(ctx context.Context) (a kaws.Auth, err error) {
	s, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return kaws.Auth{}, err
	}

	v, err := s.Credentials.Retrieve(ctx)
	if err != nil {
		return kaws.Auth{}, err
	}

	a = kaws.Auth{
		AccessKey:    v.AccessKeyID,
		SecretKey:    v.SecretAccessKey,
		SessionToken: v.SessionToken,
		UserAgent:    "crossplane-provider-kafka",
	}

	return a, nil
}

// Add options to TLS config for client certificate (if configured)
func configureClientCertificate(ctx context.Context, kc Config, kube client.Client, tc *tls.Config) error {
	sr := kc.TLS.ClientCertificateSecretRef
	if sr == nil {
		return nil
	}

	if sr.Name == "" || sr.Namespace == "" {
		return errors.New(errMissingClientCertSecretRefKeys)
	}

	secret := &corev1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{Namespace: sr.Namespace, Name: sr.Name}, secret); err != nil {
		return errors.Wrap(err, errCannotReadClientCertSecret)
	}

	kf := valueOrDefault(sr.KeyField, defaultClientCertificateKeyField)
	cf := valueOrDefault(sr.CertField, defaultClientCertificateCertField)
	kp, err := tls.X509KeyPair(secret.Data[cf], secret.Data[kf])
	if err != nil {
		return errors.Wrapf(err, "Invalid key pair, using fields %q/%q from secret %q in namespace %q",
			cf, kf, sr.Name, sr.Namespace)
	}

	tc.Certificates = append(tc.Certificates, kp)
	return nil
}

// Helper method to return default if value string is empty.
func valueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}
