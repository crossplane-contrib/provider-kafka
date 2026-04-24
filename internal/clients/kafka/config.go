package kafka

// CACertificateSecretRef is a TLS option for providing a custom CA certificate from a Kubernetes secret
type CACertificateSecretRef struct {
	CAField   string `json:"caField,omitempty"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// ClientCertificatePath is a TLS option for enabling TLS using certificate files on disk
type ClientCertificatePath struct {
	KeyFile  string `json:"keyFile"`
	CertFile string `json:"certFile"`
}

// ClientCertificateSecretRef is a TLS option for enabling mTLS
type ClientCertificateSecretRef struct {
	CertField string `json:"certField,omitempty"`
	KeyField  string `json:"keyField,omitempty"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// Config is a Kafka client configuration
type Config struct {
	Brokers []string `json:"brokers"`
	SASL    *SASL    `json:"sasl,omitempty"`
	TLS     *TLS     `json:"tls,omitempty"`
}

// SASL is an sasl option
type SASL struct {
	Mechanism string `json:"mechanism"`
	RoleArn   string `json:"roleArn"`
	Username  string `json:"username"`
	Password  string `json:"password"` //nolint:gosec
}

// TLS is an option for enabling encryption in transit.
//
// Cipher Suite Configuration:
//   - CipherSuites only applies to TLS 1.0–1.2. TLS 1.3 cipher suites are automatically
//     selected by Go's TLS implementation and cannot be overridden.
//   - Valid names are those returned by tls.CipherSuites() (e.g., TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256).
//   - TLS 1.3 suite names (e.g., TLS_AES_128_GCM_SHA256) will be rejected with a clear error.
//   - If MinVersion is set to TLS13, CipherSuites must be empty (TLS 1.3 has automatic suite selection).
//
// Version Constraints:
// - MinVersion and MaxVersion accept "TLS12" and "TLS13".
// - MinVersion must be <= MaxVersion when both are specified.
type TLS struct {
	CACertificateFile      string                  `json:"caCertificateFile,omitempty"`
	CACertificateSecretRef *CACertificateSecretRef `json:"caCertificateSecretRef,omitempty"`
	// CipherSuites lists cipher suite names for TLS 1.0–1.2 negotiation.
	// Do not include TLS 1.3 suite names (TLS_AES_*); they will be rejected.
	// If MinVersion is TLS13, this must be empty (TLS 1.3 uses automatic suite selection).
	CipherSuites               []string                    `json:"cipherSuites,omitempty"`
	ClientCertificatePath      *ClientCertificatePath      `json:"clientCertificatePath,omitempty"`
	ClientCertificateSecretRef *ClientCertificateSecretRef `json:"clientCertificateSecretRef,omitempty"`
	// ClientSessionCacheCapacity sets the LRU client session cache size. Must be >= 0; negative values are rejected.
	// 0 (default) disables session caching; positive values enable it.
	ClientSessionCacheCapacity int      `json:"clientSessionCacheCapacity,omitempty"`
	CurvePreferences           []string `json:"curvePreferences,omitempty"`
	// DialTimeoutSeconds specifies the timeout for establishing TLS connections. Must be >= 0 (negative values rejected).
	// 0 uses the default (10 seconds).
	DialTimeoutSeconds          int      `json:"dialTimeoutSeconds,omitempty"`
	DynamicRecordSizingDisabled bool     `json:"dynamicRecordSizingDisabled,omitempty"`
	InsecureSkipVerify          bool     `json:"insecureSkipVerify"`
	MaxVersion                  string   `json:"maxVersion,omitempty"`
	MinVersion                  string   `json:"minVersion,omitempty"`
	NextProtos                  []string `json:"nextProtos,omitempty"`
	ServerName                  string   `json:"serverName,omitempty"`
	SessionTicketsDisabled      bool     `json:"sessionTicketsDisabled,omitempty"`
}
