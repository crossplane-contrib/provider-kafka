package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
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
	defaultCACertificateField         = "ca.crt"
	defaultClientCertificateKeyField  = "tls.key"
	defaultClientCertificateCertField = "tls.crt"
	defaultTLSDialTimeoutSeconds      = 10

	errCannotAppendCACert             = "cannot append CA certificate to pool"
	errCannotParse                    = "cannot parse credentials"
	errCannotReadCACertFile           = "cannot read CA cert file"
	errCannotReadCACertSecret         = "cannot read CA cert secret"
	errCannotReadClientCertFile       = "cannot read client cert file"
	errCannotReadClientCertSecret     = "cannot read client cert secret"
	errInvalidCipherSuite             = "invalid cipher suite"
	errInvalidCurve                   = "invalid curve preference"
	errInvalidTLSVersion              = "invalid TLS version"
	errMissingCACertSecretRefKeys     = "missing CA cert ref secret name or namespace"
	errMissingClientCertFileKeys      = "missing client certificate keyFile or certFile"
	errMissingClientCertSecretRefKeys = "missing client cert ref secret name or namespace"
	errMissingSASLCredentials         = "SASL username and password are required"
	errMissingSASLMechanism           = "SASL mechanism is required"
)

// NewAdminClient creates a new AdminClient with supplied credentials
func NewAdminClient(ctx context.Context, data []byte, kube client.Client) (*kadm.Client, error) { // nolint: gocyclo
	kc := Config{}

	if err := json.Unmarshal(data, &kc); err != nil {
		return nil, fmt.Errorf("%s: %w", errCannotParse, err)
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
		case "scram-sha-512":
			mechanism = scram.Auth{
				User: kc.SASL.Username,
				Pass: kc.SASL.Password,
			}.AsSha512Mechanism()
		default:
			return nil, fmt.Errorf("SASL mechanism %q not supported, only PLAIN / SCRAM-SHA-512 / AWS-MSK-IAM are supported for now", kc.SASL.Mechanism)
		}
		opts = append(opts, kgo.SASL(mechanism))
	}

	// Determine dial timeout and whether TLS is needed
	dialTimeout := defaultTLSDialTimeoutSeconds
	if kc.TLS != nil && kc.TLS.DialTimeoutSeconds > 0 {
		dialTimeout = kc.TLS.DialTimeoutSeconds
	}

	isAwsMskIam := kc.SASL != nil && strings.EqualFold(kc.SASL.Mechanism, "aws-msk-iam")

	// Set dial timeout if TLS or AWS-MSK-IAM (which requires TLS)
	if kc.TLS != nil || isAwsMskIam {
		opts = append(opts, kgo.DialTimeout(time.Duration(dialTimeout)*time.Second))
	}

	// Configure TLS
	if kc.TLS != nil {
		tc := new(tls.Config)
		tc.InsecureSkipVerify = kc.TLS.InsecureSkipVerify
		if err := configureClientCertificate(ctx, kc, kube, tc); err != nil {
			return nil, err
		}
		if err := configureTLSAdvanced(kc.TLS, tc); err != nil {
			return nil, err
		}
		opts = append(opts, kgo.DialTLSConfig(tc))
	} else if isAwsMskIam {
		// AWS-MSK-IAM requires TLS; enable with default config
		opts = append(opts, kgo.DialTLS())
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
	// Validate that both clientCertificateSecretRef and clientCertificatePath are not both set.
	// In Go TLS, GetClientCertificate (used for file-based certs) takes precedence over Certificates (used for secret-based certs),
	// so setting both would silently ignore the secret-based certificate.
	if kc.TLS.ClientCertificateSecretRef != nil && kc.TLS.ClientCertificatePath != nil {
		return errors.New("cannot specify both clientCertificateSecretRef and clientCertificatePath: " +
			"use clientCertificateSecretRef for static certs or clientCertificatePath for certificates with rotation support")
	}

	if err := configureSecretRefCertificate(ctx, kc.TLS.ClientCertificateSecretRef, kube, tc); err != nil {
		return err
	}
	if err := configureFilePathCertificate(kc.TLS.ClientCertificatePath, tc); err != nil {
		return err
	}
	if err := configureCACertificateSecretRef(ctx, kc.TLS.CACertificateSecretRef, kube, tc); err != nil {
		return err
	}
	return configureCACertificateFile(kc.TLS.CACertificateFile, tc)
}

func configureSecretRefCertificate(ctx context.Context, sr *ClientCertificateSecretRef, kube client.Client, tc *tls.Config) error {
	if sr == nil {
		return nil
	}
	if sr.Name == "" || sr.Namespace == "" {
		return errors.New(errMissingClientCertSecretRefKeys)
	}

	secret := &corev1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{Namespace: sr.Namespace, Name: sr.Name}, secret); err != nil {
		return fmt.Errorf("%s: %w", errCannotReadClientCertSecret, err)
	}

	kf := valueOrDefault(sr.KeyField, defaultClientCertificateKeyField)
	cf := valueOrDefault(sr.CertField, defaultClientCertificateCertField)

	certPEM, certOk := secret.Data[cf]

	if !certOk || len(certPEM) == 0 {
		return fmt.Errorf("missing or empty client certificate field %q in secret %s/%s", cf, sr.Namespace, sr.Name)
	}

	keyPEM, keyOk := secret.Data[kf]

	if !keyOk || len(keyPEM) == 0 {
		return fmt.Errorf("missing or empty client certificate key field %q in secret %s/%s", kf, sr.Namespace, sr.Name)
	}

	kp, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("invalid key pair, using fields %q/%q from secret %q in namespace %q: %w",
			cf, kf, sr.Name, sr.Namespace, err)
	}

	tc.Certificates = append(tc.Certificates, kp)
	return nil
}

func configureFilePathCertificate(fr *ClientCertificatePath, tc *tls.Config) error {
	if fr == nil {
		return nil
	}
	if fr.KeyFile == "" || fr.CertFile == "" {
		return errors.New(errMissingClientCertFileKeys)
	}

	// Validate files exist and form valid key pair on initial load
	if err := validateClientCertificatePath(fr); err != nil {
		return err
	}

	// Use GetClientCertificate callback to support certificate rotation.
	// This allows certificates mounted via cert-manager or CSI to be reloaded
	// on each TLS handshake without restarting the provider.
	tc.GetClientCertificate = func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return loadClientCertificate(fr)
	}

	return nil
}

func validateClientCertificatePath(fr *ClientCertificatePath) error {
	// Validate that files exist and can be read initially
	if _, err := os.Stat(fr.CertFile); err != nil {
		return fmt.Errorf("%s %q: %w", errCannotReadClientCertFile, fr.CertFile, err)
	}
	if _, err := os.Stat(fr.KeyFile); err != nil {
		return fmt.Errorf("%s %q: %w", errCannotReadClientCertFile, fr.KeyFile, err)
	}

	// Validate that cert and key form a valid pair
	certPEM, err := os.ReadFile(fr.CertFile)
	if err != nil {
		return fmt.Errorf("%s %q: %w", errCannotReadClientCertFile, fr.CertFile, err)
	}
	keyPEM, err := os.ReadFile(fr.KeyFile)
	if err != nil {
		return fmt.Errorf("%s %q: %w", errCannotReadClientCertFile, fr.KeyFile, err)
	}
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return fmt.Errorf("invalid key pair, using cert file %q and key file %q: %w",
			fr.CertFile, fr.KeyFile, err)
	}
	return nil
}

func loadClientCertificate(fr *ClientCertificatePath) (*tls.Certificate, error) {
	certPEM, err := os.ReadFile(fr.CertFile)
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", errCannotReadClientCertFile, fr.CertFile, err)
	}
	keyPEM, err := os.ReadFile(fr.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("%s %q: %w", errCannotReadClientCertFile, fr.KeyFile, err)
	}

	kp, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("invalid key pair, using cert file %q and key file %q: %w",
			fr.CertFile, fr.KeyFile, err)
	}
	return &kp, nil
}

func configureCACertificateSecretRef(ctx context.Context, sr *CACertificateSecretRef, kube client.Client, tc *tls.Config) error {
	if sr == nil {
		return nil
	}
	if sr.Name == "" || sr.Namespace == "" {
		return errors.New(errMissingCACertSecretRefKeys)
	}

	secret := &corev1.Secret{}
	if err := kube.Get(ctx, types.NamespacedName{Namespace: sr.Namespace, Name: sr.Name}, secret); err != nil {
		return fmt.Errorf("%s: %w", errCannotReadCACertSecret, err)
	}

	field := valueOrDefault(sr.CAField, defaultCACertificateField)
	caPEM, ok := secret.Data[field]
	if !ok || len(caPEM) == 0 {
		return fmt.Errorf("missing or empty CA certificate field %q in secret %s/%s", field, sr.Namespace, sr.Name)
	}
	return appendCACert(caPEM, tc)
}

func configureCACertificateFile(caFile string, tc *tls.Config) error {
	if caFile == "" {
		return nil
	}

	caPEM, err := os.ReadFile(caFile) //nolint:gosec
	if err != nil {
		return fmt.Errorf("%s %q: %w", errCannotReadCACertFile, caFile, err)
	}

	return appendCACert(caPEM, tc)
}

func appendCACert(caPEM []byte, tc *tls.Config) error {
	var pool *x509.CertPool

	// Reuse existing pool if already initialized
	if tc.RootCAs != nil {
		pool = tc.RootCAs
	} else {
		// Initialize from system roots if available
		var err error
		pool, err = x509.SystemCertPool()
		if err != nil {
			// Fallback to empty pool if system roots unavailable
			pool = x509.NewCertPool()
		}
	}

	if !pool.AppendCertsFromPEM(caPEM) {
		return errors.New(errCannotAppendCACert)
	}
	tc.RootCAs = pool
	return nil
}

// Helper method to return default if value string is empty.
func valueOrDefault(value, defaultValue string) string {
	if value != "" {
		return value
	}
	return defaultValue
}

// TLS version lookup map
var tlsVersions = map[string]uint16{
	"TLS12": tls.VersionTLS12,
	"TLS13": tls.VersionTLS13,
}

// TLS curve preference lookup map
var tlsCurves = map[string]tls.CurveID{
	"P256":   tls.CurveP256,
	"P384":   tls.CurveP384,
	"P521":   tls.CurveP521,
	"X25519": tls.X25519,
}

// TLS 1.3 cipher suite names that are not user-configurable in Go's tls.Config.CipherSuites
// CipherSuites field only applies to TLS 1.0-1.2; TLS 1.3 cipher selection is automatic.
var tls13CipherSuites = map[string]bool{
	"TLS_AES_128_GCM_SHA256":       true,
	"TLS_AES_256_GCM_SHA384":       true,
	"TLS_CHACHA20_POLY1305_SHA256": true,
	"TLS_AES_128_CCM_SHA256":       true,
	"TLS_AES_128_CCM_8_SHA256":     true,
}

// buildCipherSuiteMap creates a map of cipher suite names to their IDs
// Only includes secure cipher suites from tls.CipherSuites()
func buildCipherSuiteMap() map[string]uint16 {
	m := make(map[string]uint16)
	for _, cs := range tls.CipherSuites() {
		m[cs.Name] = cs.ID
	}
	return m
}

// configureTLSVersions sets MinVersion and MaxVersion constraints
func configureTLSVersions(t *TLS, tc *tls.Config) error {
	if t.MinVersion != "" {
		version, ok := tlsVersions[t.MinVersion]
		if !ok {
			return fmt.Errorf("%s %q: valid values are TLS12, TLS13", errInvalidTLSVersion, t.MinVersion)
		}
		tc.MinVersion = version
	}

	if t.MaxVersion != "" {
		version, ok := tlsVersions[t.MaxVersion]
		if !ok {
			return fmt.Errorf("%s %q: valid values are TLS12, TLS13", errInvalidTLSVersion, t.MaxVersion)
		}
		tc.MaxVersion = version
	}

	// Validate that MinVersion <= MaxVersion when both are set
	if tc.MinVersion != 0 && tc.MaxVersion != 0 && tc.MinVersion > tc.MaxVersion {
		return fmt.Errorf("TLS MinVersion (%q) cannot be greater than MaxVersion (%q)", t.MinVersion, t.MaxVersion)
	}

	return nil
}

// configureCipherSuites sets the allowed cipher suites
func configureCipherSuites(t *TLS, tc *tls.Config) error {
	if len(t.CipherSuites) > 0 {
		cipherMap := buildCipherSuiteMap()
		suites := make([]uint16, 0, len(t.CipherSuites))
		for _, name := range t.CipherSuites {
			// Check if this is a TLS 1.3 suite (which cannot be configured in Go)
			if tls13CipherSuites[name] {
				return fmt.Errorf("%s %q: TLS 1.3 cipher suites are not configurable in Go; they are automatically selected based on protocol negotiation", errInvalidCipherSuite, name)
			}
			id, ok := cipherMap[name]
			if !ok {
				return fmt.Errorf("%s %q: valid values are from tls.CipherSuites() (TLS 1.0-1.2 only)", errInvalidCipherSuite, name)
			}
			suites = append(suites, id)
		}
		tc.CipherSuites = suites
	}
	return nil
}

// configureCurvePreferences sets the elliptic curve preferences
func configureCurvePreferences(t *TLS, tc *tls.Config) error {
	if len(t.CurvePreferences) > 0 {
		curves := make([]tls.CurveID, 0, len(t.CurvePreferences))
		for _, name := range t.CurvePreferences {
			curve, ok := tlsCurves[name]
			if !ok {
				return fmt.Errorf("%s %q: valid values are P256, P384, P521, X25519", errInvalidCurve, name)
			}
			curves = append(curves, curve)
		}
		tc.CurvePreferences = curves
	}
	return nil
}

// configureTLSAdvanced configures advanced TLS options in the tls.Config
func configureTLSAdvanced(t *TLS, tc *tls.Config) error {
	if t == nil {
		return nil
	}

	if err := configureTLSVersions(t, tc); err != nil {
		return err
	}

	if err := configureCipherSuites(t, tc); err != nil {
		return err
	}

	if err := configureCurvePreferences(t, tc); err != nil {
		return err
	}

	// Configure session ticket handling
	tc.SessionTicketsDisabled = t.SessionTicketsDisabled

	// Configure dynamic record sizing
	tc.DynamicRecordSizingDisabled = t.DynamicRecordSizingDisabled

	// Configure ALPN protocols
	if len(t.NextProtos) > 0 {
		tc.NextProtos = t.NextProtos
	}

	// Configure SNI server name
	if t.ServerName != "" {
		tc.ServerName = t.ServerName
	}

	// Configure session cache
	if t.ClientSessionCacheCapacity > 0 {
		tc.ClientSessionCache = tls.NewLRUClientSessionCache(t.ClientSessionCacheCapacity)
	}

	return nil
}
