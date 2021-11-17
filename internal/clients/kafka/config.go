package kafka

// Config is a Kafka client configuration
type Config struct {
	Brokers []string `json:"brokers"`
	SASL    *SASL    `json:"sasl,omitempty"`
}

// SASL is an sasl option
type SASL struct {
	Mechanism string `json:"mechanism"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}
