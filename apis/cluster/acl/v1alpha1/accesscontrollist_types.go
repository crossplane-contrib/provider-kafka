/*
Copyright 2025 The Crossplane Authors.

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

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	xpv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
)

// AccessControlListParameters are the configurable fields of a AccessControlList.
type AccessControlListParameters struct {
	// ResourceName is the name of the resource.
	ResourceName string `json:"resourceName"`
	// ResourceType is the type of resource.
	// Valid values are Unknown, Any, Topic, Group, Cluster, TransactionalID
	// +kubebuilder:validation:Enum=Unknown;Any;Topic;Group;Cluster;TransactionalID
	ResourceType string `json:"resourceType"`
	// ResourcePrincipal is the Principal that is being allowed or denied.
	ResourcePrincipal string `json:"resourcePrincipal"`
	// ResourceHost is the Host from which principal listed in ResourcePrinciple will have access.
	ResourceHost string `json:"resourceHost"`
	// ResourceOperation is the Operation that is being allowed or denied.
	// Valid values are Unknown, Any, All, Read, Write, Create, Delete, Alter, Describe, ClusterAction, DescribeConfigs, AlterConfigs, IdempotentWrite.
	// +kubebuilder:validation:Enum=Unknown;Any;All;Read;Write;Create;Delete;Alter;Describe;ClusterAction;DescribeConfigs;AlterConfigs;IdempotentWrite
	ResourceOperation string `json:"resourceOperation"`
	// ResourcePermissionType is the Type of permission.
	// Valid values are Unknown, Any, Allow, Deny.
	// +kubebuilder:validation:Enum=Unknown;Any;Allow;Deny
	ResourcePermissionType string `json:"resourcePermissionType"`
	// ResourcePatternTypeFilter is the pattern filter.
	// Valid values are Prefixed, Any, Match, Literal.
	// +kubebuilder:validation:Enum=Prefixed;Any;Match;Literal
	ResourcePatternTypeFilter string `json:"resourcePatternTypeFilter"`
}

// AccessControlListObservation are the observable fields of a AccessControlList.
type AccessControlListObservation struct {
	ID string `json:"id,omitempty"`
}

// A AccessControlListSpec defines the desired state of a AccessControlList.
type AccessControlListSpec struct {
	xpv2.ManagedResourceSpec `json:",inline"`
	ForProvider              AccessControlListParameters `json:"forProvider"`
}

// A AccessControlListStatus represents the observed state of a AccessControlList.
type AccessControlListStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          AccessControlListObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A AccessControlList is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,kafka}
type AccessControlList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AccessControlListSpec   `json:"spec"`
	Status AccessControlListStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AccessControlListList contains a list of AccessControlList
type AccessControlListList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AccessControlList `json:"items"`
}

// AccessControlList type metadata.
var (
	AccessControlListKind             = reflect.TypeOf(AccessControlList{}).Name()
	AccessControlListGroupKind        = schema.GroupKind{Group: Group, Kind: AccessControlListKind}.String()
	AccessControlListKindAPIVersion   = AccessControlListKind + "." + SchemeGroupVersion.String()
	AccessControlListGroupVersionKind = SchemeGroupVersion.WithKind(AccessControlListKind)
)

func init() {
	SchemeBuilder.Register(&AccessControlList{}, &AccessControlListList{})
}
