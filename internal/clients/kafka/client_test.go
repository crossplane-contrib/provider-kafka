package kafka

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var credentials struct {
	Brokers []string `json:"brokers"`
	SASL    struct {
		Mechanism string `json:"mechanism"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	} `json:"sasl"`
}

var dataTesting = []byte(os.Getenv("KAFKA_CONFIG"))

func TestNewAdminClient_ValidCredentials(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := NewAdminClient(ctx, dataTesting, nil)
	require.NoError(t, err, "expected no error getting the client, got: %v", err)
	assert.NotNil(t, client, "expected client to be non-nil")

	brokers, err := client.ListBrokers(ctx)
	require.NoError(t, err, "expected no error listing brokers, got: %v", err)
	assert.NotEmpty(t, brokers, "expected non-empty list of brokers")

}

func TestNewAdminClient_WrongCredentials(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := json.Unmarshal(dataTesting, &credentials)
	if err != nil {
		t.Fatalf("failed to unmarshal dataTesting: %v", err)
	}
	brokersFromDataTesting, err := json.Marshal(credentials.Brokers)
	if err != nil {
		t.Fatalf("failed to marshal brokers: %v", err)
	}

	badCredentials := []byte(`{
		"brokers": ` + string(brokersFromDataTesting) + `,
		"sasl": {
			"mechanism": "` + credentials.SASL.Mechanism + `",
			"username": "wrong-user",
			"password": "wrong-pass"
		}
	}`)
	client, err := NewAdminClient(ctx, badCredentials, nil)
	require.NoError(t, err, "expected no error getting the client with wrong credentials, got: %v", err)
	assert.NotNil(t, client, "expected client to be non-nil even with wrong credentials")

	brokers, err := client.ListBrokers(ctx)
	assert.Nil(t, brokers, "expected brokers to be nil on error")
	require.Error(t, err, "expected error when using a client with wrong credentials, got: %v", err)
}

func TestNewAdminClient_EmptyPassword(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	brokersFromDataTesting, err := json.Marshal(credentials.Brokers)
	if err != nil {
		t.Fatalf("failed to marshal brokers: %v", err)
	}
	data := []byte(`{
		"brokers": ` + string(brokersFromDataTesting) + `,
		"sasl": {
			"mechanism": "` + credentials.SASL.Mechanism + `",
			"username": "` + credentials.SASL.Username + `",
			"password": ""
		}
	}`)
	client, err := NewAdminClient(ctx, data, nil)
	assert.Nil(t, client, "expected client to be nil on SASL and empty password")
	require.Error(t, err, "expected error with empty password, got nil")
}

func TestNewAdminClient_MissingSASLConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	brokersFromDataTesting, err := json.Marshal(credentials.Brokers)
	if err != nil {
		t.Fatalf("failed to marshal brokers: %v", err)
	}
	data := []byte(`{ "brokers": ` + string(brokersFromDataTesting) + ` }`)
	client, err := NewAdminClient(ctx, data, nil)
	assert.Nil(t, client, "expected client to be nil on error")
	require.Error(t, err, "expected error with missing SASL config, got nil")
}

// generateTestCertificate generates a self-signed certificate and key pair for testing
func generateTestCertificate(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate RSA key")

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "test.example.com",
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}

	// Create self-signed certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err, "failed to create certificate")

	// Encode certificate to PEM
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	// Encode private key to PEM
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err, "failed to marshal private key")

	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})

	return certPEM, keyPEM
}

// TestConfigureFilePathCertificate_MissingKeyFile tests error when KeyFile is empty
func TestConfigureFilePathCertificate_MissingKeyFile(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	cp := &ClientCertificatePath{
		KeyFile:  "",
		CertFile: "/tmp/cert.crt",
	}

	err := configureFilePathCertificate(cp, tc)
	require.Error(t, err, "expected error for missing keyFile")
	require.ErrorContains(t, err, errMissingClientCertFileKeys)
	assert.Nil(t, tc.GetClientCertificate, "expected no callback to be set")
}

// TestConfigureFilePathCertificate_MissingCertFile tests error when CertFile is empty
func TestConfigureFilePathCertificate_MissingCertFile(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	cp := &ClientCertificatePath{
		KeyFile:  "/tmp/key.key",
		CertFile: "",
	}

	err := configureFilePathCertificate(cp, tc)
	require.Error(t, err, "expected error for missing certFile")
	require.ErrorContains(t, err, errMissingClientCertFileKeys)
	assert.Nil(t, tc.GetClientCertificate, "expected no callback to be set")
}

// TestConfigureFilePathCertificate_NilReference tests nil reference is handled gracefully
func TestConfigureFilePathCertificate_NilReference(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	err := configureFilePathCertificate(nil, tc)
	require.NoError(t, err, "expected no error for nil reference")
	assert.Empty(t, tc.Certificates, "expected no certificates to be added")
}

// TestConfigureFilePathCertificate_UnreadableCertFile tests error when cert file cannot be read
func TestConfigureFilePathCertificate_UnreadableCertFile(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	// Create a temp key file that exists
	_, keyPEM := generateTestCertificate(t)
	keyFile, err := os.CreateTemp("", "test-key-*.key")
	require.NoError(t, err)
	defer os.Remove(keyFile.Name())
	_, err = keyFile.Write(keyPEM)
	require.NoError(t, err)
	keyFile.Close()

	cp := &ClientCertificatePath{
		KeyFile:  keyFile.Name(),
		CertFile: "/nonexistent/path/cert.crt",
	}

	err = configureFilePathCertificate(cp, tc)
	require.Error(t, err, "expected error for unreadable cert file")
	require.ErrorContains(t, err, errCannotReadClientCertFile)
	require.ErrorContains(t, err, "/nonexistent/path/cert.crt")
	assert.Nil(t, tc.GetClientCertificate, "expected no callback to be set")
}

// TestConfigureFilePathCertificate_UnreadableKeyFile tests error when key file cannot be read
func TestConfigureFilePathCertificate_UnreadableKeyFile(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	// Create a temp cert file that we can read
	certPEM, _ := generateTestCertificate(t)
	certFile, err := os.CreateTemp("", "test-cert-*.crt")
	require.NoError(t, err)
	defer os.Remove(certFile.Name())
	_, err = certFile.Write(certPEM)
	require.NoError(t, err)
	certFile.Close()

	cp := &ClientCertificatePath{
		KeyFile:  "/nonexistent/path/key.key",
		CertFile: certFile.Name(),
	}

	err = configureFilePathCertificate(cp, tc)
	require.Error(t, err, "expected error for unreadable key file")
	require.ErrorContains(t, err, errCannotReadClientCertFile)
	require.ErrorContains(t, err, "/nonexistent/path/key.key")
	assert.Nil(t, tc.GetClientCertificate, "expected no callback to be set")
}

// TestConfigureFilePathCertificate_InvalidPEMCert tests error when cert file contains invalid PEM
func TestConfigureFilePathCertificate_InvalidPEMCert(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	_, keyPEM := generateTestCertificate(t)

	// Create temp files
	certFile, err := os.CreateTemp("", "test-cert-*.crt")
	require.NoError(t, err)
	defer os.Remove(certFile.Name())
	_, err = certFile.WriteString("not valid PEM data")
	require.NoError(t, err)
	certFile.Close()

	keyFile, err := os.CreateTemp("", "test-key-*.key")
	require.NoError(t, err)
	defer os.Remove(keyFile.Name())
	_, err = keyFile.Write(keyPEM)
	require.NoError(t, err)
	keyFile.Close()

	cp := &ClientCertificatePath{
		KeyFile:  keyFile.Name(),
		CertFile: certFile.Name(),
	}

	err = configureFilePathCertificate(cp, tc)
	require.Error(t, err, "expected error for invalid PEM certificate")
	require.ErrorContains(t, err, "invalid key pair")
	assert.Nil(t, tc.GetClientCertificate, "expected no callback to be set")
}

// TestConfigureFilePathCertificate_InvalidPEMKey tests error when key file contains invalid PEM
func TestConfigureFilePathCertificate_InvalidPEMKey(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	certPEM, _ := generateTestCertificate(t)

	// Create temp files
	certFile, err := os.CreateTemp("", "test-cert-*.crt")
	require.NoError(t, err)
	defer os.Remove(certFile.Name())
	_, err = certFile.Write(certPEM)
	require.NoError(t, err)
	certFile.Close()

	keyFile, err := os.CreateTemp("", "test-key-*.key")
	require.NoError(t, err)
	defer os.Remove(keyFile.Name())
	_, err = keyFile.WriteString("not valid PEM data")
	require.NoError(t, err)
	keyFile.Close()

	cp := &ClientCertificatePath{
		KeyFile:  keyFile.Name(),
		CertFile: certFile.Name(),
	}

	err = configureFilePathCertificate(cp, tc)
	require.Error(t, err, "expected error for invalid PEM key")
	require.ErrorContains(t, err, "invalid key pair")
	assert.Nil(t, tc.GetClientCertificate, "expected no callback to be set")
}

// TestConfigureFilePathCertificate_MismatchedKeyPair tests error when cert and key don't match
func TestConfigureFilePathCertificate_MismatchedKeyPair(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	certPEM1, _ := generateTestCertificate(t)
	_, keyPEM2 := generateTestCertificate(t)

	// Create temp files with mismatched cert and key
	certFile, err := os.CreateTemp("", "test-cert-*.crt")
	require.NoError(t, err)
	defer os.Remove(certFile.Name())
	_, err = certFile.Write(certPEM1)
	require.NoError(t, err)
	certFile.Close()

	keyFile, err := os.CreateTemp("", "test-key-*.key")
	require.NoError(t, err)
	defer os.Remove(keyFile.Name())
	_, err = keyFile.Write(keyPEM2)
	require.NoError(t, err)
	keyFile.Close()

	cp := &ClientCertificatePath{
		KeyFile:  keyFile.Name(),
		CertFile: certFile.Name(),
	}

	err = configureFilePathCertificate(cp, tc)
	require.Error(t, err, "expected error for mismatched key pair")
	require.ErrorContains(t, err, "invalid key pair")
	assert.Nil(t, tc.GetClientCertificate, "expected no callback to be set")
}

// TestConfigureFilePathCertificate_HappyPath tests successful configuration with valid cert and key
func TestConfigureFilePathCertificate_HappyPath(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	certPEM, keyPEM := generateTestCertificate(t)

	// Create temp files
	certFile, err := os.CreateTemp("", "test-cert-*.crt")
	require.NoError(t, err)
	defer os.Remove(certFile.Name())
	_, err = certFile.Write(certPEM)
	require.NoError(t, err)
	certFile.Close()

	keyFile, err := os.CreateTemp("", "test-key-*.key")
	require.NoError(t, err)
	defer os.Remove(keyFile.Name())
	_, err = keyFile.Write(keyPEM)
	require.NoError(t, err)
	keyFile.Close()

	cp := &ClientCertificatePath{
		KeyFile:  keyFile.Name(),
		CertFile: certFile.Name(),
	}

	err = configureFilePathCertificate(cp, tc)
	require.NoError(t, err, "expected no error for valid cert and key pair")
	require.NotNil(t, tc.GetClientCertificate, "expected GetClientCertificate callback to be set")

	// Verify the callback works by invoking it
	cert, err := tc.GetClientCertificate(&tls.CertificateRequestInfo{})
	require.NoError(t, err, "expected no error calling GetClientCertificate")
	require.NotNil(t, cert, "expected certificate to be returned")
	assert.NotEmpty(t, cert.Certificate, "expected certificate bytes to be present")
	assert.NotNil(t, cert.PrivateKey, "expected private key to be present")
}

// TestConfigureFilePathCertificate_CallbackSupportsCertificateRotation tests that callback reloads certs from disk
func TestConfigureFilePathCertificate_CallbackSupportsCertificateRotation(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	certPEM1, keyPEM1 := generateTestCertificate(t)

	// Create temp files
	certFile, err := os.CreateTemp("", "test-cert-*.crt")
	require.NoError(t, err)
	defer os.Remove(certFile.Name())
	_, err = certFile.Write(certPEM1)
	require.NoError(t, err)
	certFile.Close()

	keyFile, err := os.CreateTemp("", "test-key-*.key")
	require.NoError(t, err)
	defer os.Remove(keyFile.Name())
	_, err = keyFile.Write(keyPEM1)
	require.NoError(t, err)
	keyFile.Close()

	cp := &ClientCertificatePath{
		KeyFile:  keyFile.Name(),
		CertFile: certFile.Name(),
	}

	err = configureFilePathCertificate(cp, tc)
	require.NoError(t, err, "expected no error for valid cert and key pair")
	require.NotNil(t, tc.GetClientCertificate, "expected GetClientCertificate callback to be set")

	// Verify callback returns the initial cert
	cert1, err := tc.GetClientCertificate(&tls.CertificateRequestInfo{})
	require.NoError(t, err)
	require.NotNil(t, cert1)

	// Simulate certificate rotation: write new cert/key to files
	certPEM2, keyPEM2 := generateTestCertificate(t)
	err = os.WriteFile(certFile.Name(), certPEM2, 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyFile.Name(), keyPEM2, 0644)
	require.NoError(t, err)

	// Verify callback returns the new cert (proves it's reloading from disk)
	cert2, err := tc.GetClientCertificate(&tls.CertificateRequestInfo{})
	require.NoError(t, err)
	require.NotNil(t, cert2)

	// Certs should be different (different generated certificates)
	assert.NotEqual(t, cert1.Certificate[0], cert2.Certificate[0], "expected certificate to change after rotation")
}

// TestConfigureSecretRefCertificate_NilReference tests nil reference is handled gracefully
func TestConfigureSecretRefCertificate_NilReference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tc := &tls.Config{}

	err := configureSecretRefCertificate(ctx, nil, nil, tc)
	require.NoError(t, err, "expected no error for nil reference")
	assert.Empty(t, tc.Certificates, "expected no certificates to be added")
}

// TestConfigureCACertificateFile_HappyPath tests successful configuration
func TestConfigureCACertificateFile_HappyPath(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	certPEM, _ := generateTestCertificate(t)

	// Create temp file
	certFile, err := os.CreateTemp("", "test-ca-*.crt")
	require.NoError(t, err)
	defer os.Remove(certFile.Name())
	_, err = certFile.Write(certPEM)
	require.NoError(t, err)
	certFile.Close()

	err = configureCACertificateFile(certFile.Name(), tc)
	require.NoError(t, err, "expected no error for valid CA file")
	assert.NotNil(t, tc.RootCAs, "expected RootCAs to be set")
}

// TestConfigureCACertificateFile_NonexistentFile tests error when file doesn't exist
func TestConfigureCACertificateFile_NonexistentFile(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	err := configureCACertificateFile("/nonexistent/path/ca.crt", tc)
	require.Error(t, err, "expected error for nonexistent file")
	require.ErrorContains(t, err, errCannotReadCACertFile)
}

// TestConfigureCACertificateFile_EmptyPath tests no-op when path is empty
func TestConfigureCACertificateFile_EmptyPath(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{}

	err := configureCACertificateFile("", tc)
	require.NoError(t, err, "expected no error for empty path")
	assert.Nil(t, tc.RootCAs, "expected RootCAs to remain nil")
}

// TestAppendCACert_ReuseExistingPool tests that existing pool is reused
func TestAppendCACert_ReuseExistingPool(t *testing.T) {
	t.Parallel()

	certPEM1, _ := generateTestCertificate(t)
	certPEM2, _ := generateTestCertificate(t)

	tc := &tls.Config{}

	// Add first cert
	err := appendCACert(certPEM1, tc)
	require.NoError(t, err)
	firstPool := tc.RootCAs

	// Add second cert - should reuse pool
	err = appendCACert(certPEM2, tc)
	require.NoError(t, err)

	assert.Same(t, firstPool, tc.RootCAs, "expected pool to be reused (same pointer)")
}

// TestAppendCACert_SystemRootsFallback tests fallback to system roots
func TestAppendCACert_SystemRootsFallback(t *testing.T) {
	t.Parallel()

	certPEM, _ := generateTestCertificate(t)

	tc := &tls.Config{}

	err := appendCACert(certPEM, tc)
	require.NoError(t, err)

	// Pool should be set (either from system or empty)
	assert.NotNil(t, tc.RootCAs, "expected RootCAs to be set")
}

// TestConfigureClientCertificate_MutualExclusivity tests that both cert options cannot be set
func TestConfigureClientCertificate_MutualExclusivity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	config := Config{
		TLS: &TLS{
			ClientCertificateSecretRef: &ClientCertificateSecretRef{
				Name:      "secret",
				Namespace: "default",
			},
			ClientCertificatePath: &ClientCertificatePath{
				CertFile: "/path/to/cert",
				KeyFile:  "/path/to/key",
			},
		},
	}

	tc := &tls.Config{}

	err := configureClientCertificate(ctx, config, nil, tc)
	require.Error(t, err, "expected error when both cert options are set")
	require.ErrorContains(t, err, "cannot specify both")
	require.ErrorContains(t, err, "clientCertificateSecretRef")
	require.ErrorContains(t, err, "clientCertificatePath")
}
