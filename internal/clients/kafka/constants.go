package kafka

import (
	"crypto/tls"
	"time"
)

const (
	defaultIAMCredentialsExpiryWindow = 5 * time.Minute

	// ACL resource types
	ACLResourceTypeTopic           = "Topic"
	ACLResourceTypeGroup           = "Group"
	ACLResourceTypeTransactionalID = "TransactionalID"
	ACLResourceTypeCluster         = "Cluster"
	ACLResourceTypeAny             = "Any"

	// ACL operations
	ACLOperationAlterConfigs = "AlterConfigs"
	ACLOperationRead         = "Read"
	ACLOperationWrite        = "Write"
	ACLOperationDescribe     = "Describe"

	// ACL permission and pattern types
	ACLPermissionTypeAllow = "Allow"
	ACLPatternTypeLiteral  = "Literal"

	// default Secret field names for TLS certificates, like managed by cert-manager
	defaultCACertificateField         = "ca.crt"
	defaultClientCertificateKeyField  = "tls.key"
	defaultClientCertificateCertField = "tls.crt"
	defaultTLSDialTimeoutSeconds      = 10

	tlsVersion12 = "TLS12"
	tlsVersion13 = "TLS13"

	defaultLogLevel = 2

	errInvalidLogLevel                = "invalid log level"
	errMissingBrokers                 = "at least one broker address is required"
	errCannotAppendCACert             = "cannot append CA certificate to pool"
	errCannotParse                    = "cannot parse credentials"
	errCannotReadCACertFile           = "cannot read CA cert file"
	errCannotReadCACertSecret         = "cannot read CA cert secret"
	errCannotReadClientCertFile       = "cannot read client cert file"
	errCannotReadClientCertSecret     = "cannot read client cert secret"
	errInvalidCipherSuite             = "invalid cipher suite"
	errInvalidCipherSuiteTLS13        = "cipherSuites cannot be configured in TLS13"
	errInvalidClientSessionCache      = "invalid client session cache capacity: must be >= 0"
	errInvalidCurve                   = "invalid curve preference"
	errInvalidDialTimeout             = "invalid dial timeout: must be >= 0"
	errInvalidTLSVersion              = "invalid TLS version"
	errMissingCACertSecretRefKeys     = "missing CA cert ref secret name or namespace"
	errMissingClientCertFileKeys      = "missing client certificate keyFile or certFile"
	errMissingClientCertSecretRefKeys = "missing client cert ref secret name or namespace"
	errMissingSASLCredentials         = "SASL username and password are required"
	errMissingSASLMechanism           = "SASL mechanism is required"
)

// cipherSuiteMap caches the mapping of cipher suite names to IDs.
// Only includes secure cipher suites from tls.CipherSuites() (TLS 1.0-1.2 only).
var cipherSuiteMap = func() map[string]uint16 {
	m := make(map[string]uint16)
	for _, cs := range tls.CipherSuites() {
		m[cs.Name] = cs.ID
	}
	return m
}()

// TLS 1.3 cipher suite names that are not user-configurable in Go's tls.Config.CipherSuites
// CipherSuites field only applies to TLS 1.0-1.2; TLS 1.3 cipher selection is automatic.
var tls13CipherSuites = map[string]bool{
	"TLS_AES_128_GCM_SHA256":       true,
	"TLS_AES_256_GCM_SHA384":       true,
	"TLS_CHACHA20_POLY1305_SHA256": true,
	"TLS_AES_128_CCM_SHA256":       true,
	"TLS_AES_128_CCM_8_SHA256":     true,
}

// TLS curve preference lookup map
var tlsCurves = map[string]tls.CurveID{
	"P256":   tls.CurveP256,
	"P384":   tls.CurveP384,
	"P521":   tls.CurveP521,
	"X25519": tls.X25519,
}

// TLS version lookup map
var tlsVersions = map[string]uint16{
	tlsVersion12: tls.VersionTLS12,
	tlsVersion13: tls.VersionTLS13,
}

// Test constant values used across test files
const (
	TestACLName      = "acl1"
	TestACLPrincipal = "User:Ken"
	TestTopicName1   = "testTopic-1"
)
