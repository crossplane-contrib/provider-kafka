package v1alpha1

// ConnectorState is the desired lifecycle state of a connector.
// +kubebuilder:validation:Enum=running;paused;stopped
type ConnectorState string

// Connector lifecycle states.
const (
	ConnectorStateRunning ConnectorState = "running"
	ConnectorStatePaused  ConnectorState = "paused"
	ConnectorStateStopped ConnectorState = "stopped"
)

// ConnectorParameters are the configurable fields of a Connector.
type ConnectorParameters struct {
	// Class is the connector implementation class (connector.class).
	// +kubebuilder:validation:MinLength=1
	Class string `json:"class"`

	// TasksMax is the maximum number of tasks for the connector (tasks.max).
	// +kubebuilder:validation:Minimum=1
	// +optional
	TasksMax *int32 `json:"tasksMax,omitempty"`

	// Config holds the remaining connector properties. Values are stored in
	// plain text, so reference credentials through Kafka Connect config
	// providers, for example ${file:/path:key} or ${env:VAR}.
	// +kubebuilder:validation:XValidation:rule="!('connector.class' in self) && !('tasks.max' in self) && !('name' in self)",message="set connector.class, tasks.max and name through class, tasksMax and the external name"
	// +optional
	Config map[string]string `json:"config,omitempty"`

	// State is the desired lifecycle state of the connector.
	// +kubebuilder:default=running
	// +optional
	State ConnectorState `json:"state,omitempty"`
}

// ConnectorObservation are the observable fields of a Connector.
type ConnectorObservation struct {
	// State is the connector state reported by Kafka Connect.
	// +optional
	State string `json:"state,omitempty"`

	// WorkerID is the Connect worker running the connector.
	// +optional
	WorkerID string `json:"workerId,omitempty"`

	// Tasks lists the state of each connector task.
	// +optional
	Tasks []TaskStatus `json:"tasks,omitempty"`
}

// TaskStatus is the observed state of one connector task.
type TaskStatus struct {
	// ID is the task number.
	ID int32 `json:"id"`

	// State is the task state reported by Kafka Connect.
	State string `json:"state"`

	// WorkerID is the Connect worker running the task.
	// +optional
	WorkerID string `json:"workerId,omitempty"`
}
