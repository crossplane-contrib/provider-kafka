package kafka

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

// TLS is an option for enabling encryption in transit
type TLS struct {
	ClientCertificateSecretRef *ClientCertificateSecretRef `json:"clientCertificateSecretRef,omitempty"`
	ClientCertificatePath      *ClientCertificatePath      `json:"clientCertificatePath,omitempty"`
	CACertificateSecretRef     *CACertificateSecretRef     `json:"caCertificateSecretRef,omitempty"`
	CACertificateFile          string                      `json:"caCertificateFile,omitempty"`
	InsecureSkipVerify         bool                        `json:"insecureSkipVerify"`
}

// ClientCertificateSecretRef is a TLS option for enable mTLS
type ClientCertificateSecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	KeyField  string `json:"keyField,omitempty"`
	CertField string `json:"certField,omitempty"`
}

// ClientCertificatePath is a TLS option for enabling TLS using certificate files on disk
type ClientCertificatePath struct {
	KeyFile  string `json:"keyFile"`
	CertFile string `json:"certFile"`
}

// CACertificateSecretRef is a TLS option for providing a custom CA certificate from a Kubernetes secret
type CACertificateSecretRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	CAField   string `json:"caField,omitempty"`
}
