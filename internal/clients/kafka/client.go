package kafka

import (
	"context"
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// default Secret field names for TLS certificates, like managed by cert-manager
	defaultClientCertificateKeyField  = "tls.key"
	defaultClientCertificateCertField = "tls.crt"

	errCannotParse                    = "cannot parse credentials"
	errMissingClientCertSecretRefKeys = "missing client cert ref secret name or namespace"
	errCannotReadClientCertSecret     = "cannot read client cert secret"
)

// NewAdminClient creates a new AdminClient with supplied credentials
func NewAdminClient(ctx context.Context, data []byte, kube client.Client) (*kadm.Client, error) {
	kc := Config{}

	if err := json.Unmarshal(data, &kc); err != nil {
		return nil, errors.Wrap(err, errCannotParse)
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
		tc.InsecureSkipVerify = kc.TLS.InsecureSkipVerify

		if sr := kc.TLS.ClientCertificateSecretRef; sr != nil {
			if sr.Name == "" || sr.Namespace == "" {
				return nil, errors.New(errMissingClientCertSecretRefKeys)
			}
			secret := &corev1.Secret{}
			if err := kube.Get(ctx, types.NamespacedName{Namespace: sr.Namespace, Name: sr.Name}, secret); err != nil {
				return nil, errors.Wrap(err, errCannotReadClientCertSecret)
			}
			kf := valueOrDefault(sr.KeyField, defaultClientCertificateKeyField)
			cf := valueOrDefault(sr.CertField, defaultClientCertificateCertField)
			kp, err := tls.X509KeyPair(secret.Data[cf], secret.Data[kf])
			if err != nil {
				return nil, errors.Wrapf(err, "Invalid key pair, using fields %q/%q from secret %q in namespace %q",
					cf, kf, sr.Name, sr.Namespace)
			}
			tc.Certificates = append(tc.Certificates, kp)
		}
		opts = append(opts, kgo.DialTLSConfig(tc))
	}

	c, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return kadm.NewClient(c), nil
}

// Helper method to return default if value string is empty.
func valueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}
