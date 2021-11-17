/*
Copyright 2020 The Crossplane Authors.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// TopicParameters are the configurable fields of a Topic.
type TopicParameters struct {
	ReplicationFactor int `json:"replicationFactor"`
	Partitions        int `json:"partitions"`
}

// TopicObservation are the observable fields of a Topic.
type TopicObservation struct {
	ID string `json:"id,omitempty"`
}

// A TopicSpec defines the desired state of a Topic.
type TopicSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       TopicParameters `json:"forProvider"`
}

// A TopicStatus represents the observed state of a Topic.
type TopicStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          TopicObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Topic is an example API type.
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,kafka}
type Topic struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TopicSpec   `json:"spec"`
	Status TopicStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TopicList contains a list of Topic
type TopicList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Topic `json:"items"`
}

// Topic type metadata.
var (
	TopicKind             = reflect.TypeOf(Topic{}).Name()
	TopicGroupKind        = schema.GroupKind{Group: Group, Kind: TopicKind}.String()
	TopicKindAPIVersion   = TopicKind + "." + SchemeGroupVersion.String()
	TopicGroupVersionKind = SchemeGroupVersion.WithKind(TopicKind)
)

func init() {
	SchemeBuilder.Register(&Topic{}, &TopicList{})
}
