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

// AclParameters are the configurable fields of a Acl.
type AclParameters struct {
	// +kubebuilder:validation:Enum=Unknown;Any;Topic;Group;Cluster;TransactionalID
	ResourceType      string `json:"resourceType"`
	AclPrinciple      string `json:"aclPrinciple"`
	AclHost           string `json:"aclHost"`
	// +kubebuilder:validation:Enum=Unknown;Any;All;Read;Write;Create;Delete;Alter;Describe;ClusterAction;DescribeConfigs;AlterConfigs;IdempotentWrite
	AclOperation      string `json:"aclOperation"`
	// +kubebuilder:validation:Enum=Unknown;Any;Allow;Deny
	AclPermissionType string `json:"aclPermissionType"`
	// +kubebuilder:validation:Enum=Prefixed;Any;Match;Literal
	ResourcePatternTypeFilter string `json:"resourcePatternTypeFilter"`
}

// AclObservation are the observable fields of an Acl
type AclObservation struct {
	ID string `json:"id,omitempty"`
}

// An AclSpec defines the desired state of an Acl
type AclSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       AclParameters `json:"forProvider"`
}

// A AclStatus represents the observed state of a Acl.
type AclStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          AclObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Acl is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,kafka}
type Acl struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AclSpec   `json:"spec"`
	Status AclStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AclList contains a list of Acl
type AclList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Acl `json:"items"`
}

// Acl type metadata.
var (
	AclKind             = reflect.TypeOf(Acl{}).Name()
	AclGroupKind        = schema.GroupKind{Group: Group, Kind: AclKind}.String()
	AclKindAPIVersion   = AclKind + "." + SchemeGroupVersion.String()
	AclGroupVersionKind = SchemeGroupVersion.WithKind(AclKind)
)

func init() {
	SchemeBuilder.Register(&Acl{}, &AclList{})
}
