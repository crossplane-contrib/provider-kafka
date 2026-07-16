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

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UserObservation) DeepCopyInto(out *UserObservation) {
	*out = *in
	if in.Mechanisms != nil {
		in, out := &in.Mechanisms, &out.Mechanisms
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new UserObservation.
func (in *UserObservation) DeepCopy() *UserObservation {
	if in == nil {
		return nil
	}
	out := new(UserObservation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UserParameters) DeepCopyInto(out *UserParameters) {
	*out = *in
	if in.Mechanisms != nil {
		in, out := &in.Mechanisms, &out.Mechanisms
		*out = make([]Mechanism, len(*in))
		copy(*out, *in)
	}
	if in.PasswordSecretRef != nil {
		in, out := &in.PasswordSecretRef, &out.PasswordSecretRef
		*out = new(SecretKeySelector)
		**out = **in
	}
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new UserParameters.
func (in *UserParameters) DeepCopy() *UserParameters {
	if in == nil {
		return nil
	}
	out := new(UserParameters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecretKeySelector) DeepCopyInto(out *SecretKeySelector) {
	*out = *in
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new SecretKeySelector.
func (in *SecretKeySelector) DeepCopy() *SecretKeySelector {
	if in == nil {
		return nil
	}
	out := new(SecretKeySelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedSecretKeySelector) DeepCopyInto(out *NamespacedSecretKeySelector) {
	*out = *in
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new NamespacedSecretKeySelector.
func (in *NamespacedSecretKeySelector) DeepCopy() *NamespacedSecretKeySelector {
	if in == nil {
		return nil
	}
	out := new(NamespacedSecretKeySelector)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NamespacedUserParameters) DeepCopyInto(out *NamespacedUserParameters) {
	*out = *in
	if in.Mechanisms != nil {
		in, out := &in.Mechanisms, &out.Mechanisms
		*out = make([]Mechanism, len(*in))
		copy(*out, *in)
	}
	if in.PasswordSecretRef != nil {
		in, out := &in.PasswordSecretRef, &out.PasswordSecretRef
		*out = new(NamespacedSecretKeySelector)
		**out = **in
	}
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new NamespacedUserParameters.
func (in *NamespacedUserParameters) DeepCopy() *NamespacedUserParameters {
	if in == nil {
		return nil
	}
	out := new(NamespacedUserParameters)
	in.DeepCopyInto(out)
	return out
}
