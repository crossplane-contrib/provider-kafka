/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package connector holds logic shared by the cluster and namespaced
// Connector controllers.
package connector

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"

	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/connect"
)

// Error messages shared by both Connector controllers.
const (
	ErrNotConnector = "managed resource is not a Connector custom resource"
	ErrParseCreds   = "cannot parse provider credentials"
	ErrNewClient    = "cannot create Kafka Connect client"
	ErrObserve      = "cannot observe connector"
	ErrPutConfig    = "cannot put connector config"
	ErrSetState     = "cannot change connector state"
	ErrDelete       = "cannot delete connector"
)

// Connector config keys managed through dedicated spec fields.
const (
	KeyClass    = "connector.class"
	KeyName     = "name"
	KeyTasksMax = "tasks.max"
)

// Kafka Connect connector and task states.
const (
	stateRunning = "RUNNING"
	statePaused  = "PAUSED"
	stateStopped = "STOPPED"
)

// Desired returns the connector config to send to Kafka Connect. Kafka
// Connect stores "name" alongside the submitted config, so it is included
// to compare equal with the observed config.
func Desired(p commonv1alpha1.ConnectorParameters, name string) map[string]string {
	out := maps.Clone(p.Config)
	if out == nil {
		out = make(map[string]string, 3)
	}
	out[KeyClass] = p.Class
	out[KeyName] = name
	if p.TasksMax != nil {
		out[KeyTasksMax] = strconv.Itoa(int(*p.TasksMax))
	}
	return out
}

// UpToDate reports whether the observed config equals the desired config.
// Kafka Connect returns the config as stored, without defaults or masking, so
// any difference, including a removed key, is drift.
func UpToDate(desired, observed map[string]string) bool {
	return maps.Equal(desired, observed)
}

// StateUpToDate reports whether the connector is in the desired state. Only
// RUNNING, PAUSED and STOPPED can be changed through the REST API, so FAILED
// and UNASSIGNED never count as drift; Condition reports them instead.
func StateUpToDate(desired commonv1alpha1.ConnectorState, s *connect.Status) bool {
	switch s.Connector.State {
	case stateRunning, statePaused, stateStopped:
		return s.Connector.State == wantState(desired)
	}
	return true
}

// StateAction returns the REST action that moves a connector to desired.
func StateAction(desired commonv1alpha1.ConnectorState) string {
	switch desired {
	case commonv1alpha1.ConnectorStatePaused:
		return connect.ActionPause
	case commonv1alpha1.ConnectorStateStopped:
		return connect.ActionStop
	case commonv1alpha1.ConnectorStateRunning:
		return connect.ActionResume
	default:
		return connect.ActionResume
	}
}

// Condition returns Available when the connector and all of its tasks are in
// the desired state, and Unavailable with the first mismatch otherwise.
// Task stack traces are left out of the message.
func Condition(desired commonv1alpha1.ConnectorState, s *connect.Status) xpv2.Condition {
	want := wantState(desired)
	if s.Connector.State != want {
		return xpv2.Unavailable().WithMessage(mismatch("connector", s.Connector.State, want))
	}
	for _, t := range s.Tasks {
		if t.State != want {
			return xpv2.Unavailable().WithMessage(mismatch(fmt.Sprintf("task %d", t.ID), t.State, want))
		}
	}
	return xpv2.Available()
}

// Observation converts a Connect status into the resource's atProvider.
func Observation(s *connect.Status) commonv1alpha1.ConnectorObservation {
	o := commonv1alpha1.ConnectorObservation{State: s.Connector.State, WorkerID: s.Connector.WorkerID}
	for _, t := range s.Tasks {
		o.Tasks = append(o.Tasks, commonv1alpha1.TaskStatus{ID: t.ID, State: t.State, WorkerID: t.WorkerID})
	}
	return o
}

func wantState(desired commonv1alpha1.ConnectorState) string {
	if desired == "" {
		return stateRunning
	}
	return strings.ToUpper(string(desired))
}

func mismatch(what, got, want string) string {
	return fmt.Sprintf("%s is %s, want %s", what, got, want)
}
