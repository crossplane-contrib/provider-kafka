package kafka

type Config struct {
	Brokers []string `json:"brokers"`
	SASL    *SASL    `json:"sasl,omitempty"`
}

type SASL struct {
	Mechanism string `json:"mechanism"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}
