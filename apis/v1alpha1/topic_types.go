package v1alpha1

// TopicObservation are the observable fields of a Topic.
type TopicObservation struct {
	ID string `json:"id,omitempty"`
	// ReplicationFactor is the observed number of replicas for the topic.
	ReplicationFactor int `json:"replicationFactor,omitempty"`
	// Partitions is the observed number of partitions for the topic.
	Partitions int `json:"partitions,omitempty"`
	// Config is the observed topic configuration from Kafka.
	// +optional
	Config map[string]*string `json:"config,omitempty"`
}

// TopicParameters are the configurable fields of a Topic.
type TopicParameters struct {
	// ReplicationFactor defines the number of replicas the topic should have.
	// +kubebuilder:validation:Minimum:=1
	ReplicationFactor int `json:"replicationFactor"`
	// Partitions defines the number of partitions the topic should have.
	// +kubebuilder:validation:Minimum:=1
	Partitions int `json:"partitions"`
	// Config is an optional map of string key/ value pairs.
	// +optional
	Config map[string]*string `json:"config,omitempty"`
}
