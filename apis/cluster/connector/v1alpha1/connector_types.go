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

package v1alpha1

import (
	"reflect"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	common "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
)

// A ConnectorSpec defines the desired state of a Connector.
type ConnectorSpec struct {
	xpv2.ClusterManagedResourceSpec `json:",inline"`
	ForProvider                     common.ConnectorParameters `json:"forProvider"`
}

// A ConnectorStatus represents the observed state of a Connector.
type ConnectorStatus struct {
	xpv2.ManagedResourceStatus `json:",inline"`
	AtProvider                 common.ConnectorObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Connector is a Kafka Connect connector managed resource.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="STATE",type="string",JSONPath=".status.atProvider.state"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,kafka}
type Connector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConnectorSpec   `json:"spec"`
	Status ConnectorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConnectorList contains a list of Connector
type ConnectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Connector `json:"items"`
}

// Connector type metadata.
var (
	ConnectorKind             = reflect.TypeOf(Connector{}).Name()
	ConnectorGroupKind        = schema.GroupKind{Group: Group, Kind: ConnectorKind}.String()
	ConnectorKindAPIVersion   = ConnectorKind + "." + SchemeGroupVersion.String()
	ConnectorGroupVersionKind = SchemeGroupVersion.WithKind(ConnectorKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Connector{}, &ConnectorList{})
		return nil
	})
}
