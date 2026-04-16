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
	Password  string `json:"password"` //nolint:gosec
	Username  string `json:"username"`
}

// TLS is an option for enabling encryption in transit
type TLS struct {
	CACertificateFile           string                      `json:"caCertificateFile,omitempty"`
	CACertificateSecretRef      *CACertificateSecretRef     `json:"caCertificateSecretRef,omitempty"`
	CipherSuites                []string                    `json:"cipherSuites,omitempty"`
	ClientCertificatePath       *ClientCertificatePath      `json:"clientCertificatePath,omitempty"`
	ClientCertificateSecretRef  *ClientCertificateSecretRef `json:"clientCertificateSecretRef,omitempty"`
	ClientSessionCacheCapacity  int                         `json:"clientSessionCacheCapacity,omitempty"`
	CurvePreferences            []string                    `json:"curvePreferences,omitempty"`
	DialTimeoutSeconds          int                         `json:"dialTimeoutSeconds,omitempty"`
	DynamicRecordSizingDisabled bool                        `json:"dynamicRecordSizingDisabled,omitempty"`
	InsecureSkipVerify          bool                        `json:"insecureSkipVerify"`
	MaxVersion                  string                      `json:"maxVersion,omitempty"`
	MinVersion                  string                      `json:"minVersion,omitempty"`
	NextProtos                  []string                    `json:"nextProtos,omitempty"`
	ServerName                  string                      `json:"serverName,omitempty"`
	SessionTicketsDisabled      bool                        `json:"sessionTicketsDisabled,omitempty"`
}
