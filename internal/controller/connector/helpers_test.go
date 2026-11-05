package connector

import (
	"testing"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/connect"
)

const keyTopics = "topics"

func status(connector string, tasks ...string) *connect.Status {
	s := &connect.Status{}
	s.Connector.State = connector
	s.Connector.WorkerID = "w0"
	for i, t := range tasks {
		s.Tasks = append(s.Tasks, connect.TaskStatus{ID: int32(i), State: t, WorkerID: "w1"})
	}
	return s
}

func TestDesired(t *testing.T) {
	t.Parallel()

	two := int32(2)
	cases := map[string]struct {
		in   commonv1alpha1.ConnectorParameters
		want map[string]string
	}{
		"ClassOnly": {
			in:   commonv1alpha1.ConnectorParameters{Class: "X"},
			want: map[string]string{KeyClass: "X", KeyName: "c"},
		},
		"AllFields": {
			in: commonv1alpha1.ConnectorParameters{
				Class:    "X",
				TasksMax: &two,
				Config:   map[string]string{keyTopics: "orders"},
			},
			want: map[string]string{KeyClass: "X", "tasks.max": "2", keyTopics: "orders", KeyName: "c"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, Desired(tc.in, "c"))
		})
	}
}

func TestDesired_DoesNotMutateSpec(t *testing.T) {
	t.Parallel()

	cfg := map[string]string{keyTopics: "orders"}
	_ = Desired(commonv1alpha1.ConnectorParameters{Class: "X", Config: cfg}, "c")
	assert.Equal(t, map[string]string{keyTopics: "orders"}, cfg)
}

func TestUpToDate(t *testing.T) {
	t.Parallel()

	desired := map[string]string{KeyClass: "X", KeyName: "c", keyTopics: "a"}
	cases := map[string]struct {
		observed map[string]string
		want     bool
	}{
		"Equal":      {observed: map[string]string{KeyClass: "X", KeyName: "c", keyTopics: "a"}, want: true},
		"Changed":    {observed: map[string]string{KeyClass: "X", KeyName: "c", keyTopics: "b"}},
		"KeyAdded":   {observed: map[string]string{KeyClass: "X", KeyName: "c", keyTopics: "a", "extra": "1"}},
		"KeyRemoved": {observed: map[string]string{KeyClass: "X", KeyName: "c"}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, UpToDate(desired, tc.observed))
		})
	}
}

func TestStateUpToDate(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		desired  commonv1alpha1.ConnectorState
		observed string
		want     bool
	}{
		"RunningRunning":    {desired: commonv1alpha1.ConnectorStateRunning, observed: "RUNNING", want: true},
		"DefaultIsRunning":  {desired: "", observed: "RUNNING", want: true},
		"RunningPaused":     {desired: commonv1alpha1.ConnectorStateRunning, observed: "PAUSED"},
		"PausedPaused":      {desired: commonv1alpha1.ConnectorStatePaused, observed: "PAUSED", want: true},
		"StoppedRunning":    {desired: commonv1alpha1.ConnectorStateStopped, observed: "RUNNING"},
		"FailedIsIgnored":   {desired: commonv1alpha1.ConnectorStateRunning, observed: "FAILED", want: true},
		"UnassignedIgnored": {desired: commonv1alpha1.ConnectorStatePaused, observed: "UNASSIGNED", want: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, StateUpToDate(tc.desired, status(tc.observed)))
		})
	}
}

func TestStateAction(t *testing.T) {
	t.Parallel()

	assert.Equal(t, connect.ActionResume, StateAction(""))
	assert.Equal(t, connect.ActionResume, StateAction(commonv1alpha1.ConnectorStateRunning))
	assert.Equal(t, connect.ActionPause, StateAction(commonv1alpha1.ConnectorStatePaused))
	assert.Equal(t, connect.ActionStop, StateAction(commonv1alpha1.ConnectorStateStopped))
}

func TestCondition(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		desired    commonv1alpha1.ConnectorState
		st         *connect.Status
		wantStatus corev1.ConditionStatus
		wantMsg    string
	}{
		"AllRunning": {
			st:         status("RUNNING", "RUNNING", "RUNNING"),
			wantStatus: corev1.ConditionTrue,
		},
		"RunningNoTasks": {
			st:         status("RUNNING"),
			wantStatus: corev1.ConditionTrue,
		},
		"TaskFailed": {
			st:         status("RUNNING", "RUNNING", "FAILED"),
			wantStatus: corev1.ConditionFalse,
			wantMsg:    "task 1 is FAILED",
		},
		"ConnectorFailed": {
			st:         status("FAILED"),
			wantStatus: corev1.ConditionFalse,
			wantMsg:    "connector is FAILED",
		},
		"PausedAsDesired": {
			desired:    commonv1alpha1.ConnectorStatePaused,
			st:         status("PAUSED", "PAUSED"),
			wantStatus: corev1.ConditionTrue,
		},
		"StoppedAsDesired": {
			desired:    commonv1alpha1.ConnectorStateStopped,
			st:         status("STOPPED"),
			wantStatus: corev1.ConditionTrue,
		},
		"StillPausing": {
			desired:    commonv1alpha1.ConnectorStatePaused,
			st:         status("PAUSED", "RUNNING"),
			wantStatus: corev1.ConditionFalse,
			wantMsg:    "task 0 is RUNNING, want PAUSED",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := Condition(tc.desired, tc.st)
			assert.Equal(t, xpv2.TypeReady, got.Type)
			assert.Equal(t, tc.wantStatus, got.Status)
			assert.Contains(t, got.Message, tc.wantMsg)
		})
	}
}

func TestObservation(t *testing.T) {
	t.Parallel()

	got := Observation(status("RUNNING", "FAILED"))
	assert.Equal(t, commonv1alpha1.ConnectorObservation{
		State:    "RUNNING",
		WorkerID: "w0",
		Tasks:    []commonv1alpha1.TaskStatus{{ID: 0, State: "FAILED", WorkerID: "w1"}},
	}, got)
}
