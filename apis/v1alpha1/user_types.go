package v1alpha1

// Mechanism is a Kafka SCRAM mechanism name.
// +kubebuilder:validation:Enum="SCRAM-SHA-256";"SCRAM-SHA-512"
type Mechanism string

// UserObservation are the observable fields of a User.
type UserObservation struct {
	// Mechanisms lists the SCRAM mechanisms currently enrolled for this user in Kafka.
	// +optional
	Mechanisms []string `json:"mechanisms,omitempty"`
}

// SecretKeySelector selects a key from a Kubernetes Secret.
// Used by cluster-scoped resources where the namespace must be explicit.
type SecretKeySelector struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// Namespace is the namespace of the Secret.
	Namespace string `json:"namespace"`
	// Key is the key within the Secret's data map.
	Key string `json:"key"`
}

// NamespacedSecretKeySelector selects a key from a Kubernetes Secret in the
// same namespace as the referencing resource. Cross-namespace references are
// not permitted for namespace-scoped resources.
type NamespacedSecretKeySelector struct {
	// Name is the name of the Secret.
	Name string `json:"name"`
	// Key is the key within the Secret's data map.
	Key string `json:"key"`
}

// UserParameters are the configurable fields of a cluster-scoped User.
type UserParameters struct {
	// Mechanisms lists the SCRAM mechanisms to enroll the user in.
	// Valid values are SCRAM-SHA-256 and SCRAM-SHA-512.
	// +kubebuilder:default={"SCRAM-SHA-512"}
	// +optional
	Mechanisms []Mechanism `json:"mechanisms,omitempty"`

	// PasswordSecretRef is an optional reference to a Kubernetes Secret
	// containing the user's password. When set, the controller reads the
	// password from the specified key. When omitted, the controller auto-generates
	// a secure random password and persists it in the connection Secret.
	// +optional
	PasswordSecretRef *SecretKeySelector `json:"passwordSecretRef,omitempty"`
}

// NamespacedUserParameters are the configurable fields of a namespaced User.
// The password Secret reference does not include a namespace — the Secret must
// reside in the same namespace as the User resource.
type NamespacedUserParameters struct {
	// Mechanisms lists the SCRAM mechanisms to enroll the user in.
	// Valid values are SCRAM-SHA-256 and SCRAM-SHA-512.
	// +kubebuilder:default={"SCRAM-SHA-512"}
	// +optional
	Mechanisms []Mechanism `json:"mechanisms,omitempty"`

	// PasswordSecretRef is an optional reference to a Kubernetes Secret in the
	// same namespace as this User. When set, the controller reads the password
	// from the specified key. When omitted, the controller auto-generates a
	// secure random password and persists it in the connection Secret.
	// +optional
	PasswordSecretRef *NamespacedSecretKeySelector `json:"passwordSecretRef,omitempty"`
}
