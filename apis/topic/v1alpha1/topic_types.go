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
	// +kubebuilder:validation:Minimum:=1
	ReplicationFactor int `json:"replicationFactor"`
	// +kubebuilder:validation:Minimum:=1
	Partitions int `json:"partitions"`
	// +optional
	// +kubebuilder:validation:Enum:=compact;delete
	CleanupPolicy []metav1.List `json:"cleanupPolicy"`
	// +optional
	// +kubebuilder:validation:Enum:=uncompressed;zstd;lz4;snappy;gzip;producer
	CompressionType *string `json:"compressionType"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	DeleteRetentionMs *int64 `json:"deleteRetentionMs"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	FileDeleteDelayMs *int64 `json:"fileDeleteDelayMs"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	FlushMessages *int64 `json:"flushMessages"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	FlushMs *int64 `json:"flushMs"`
	// +optional
	FollowerReplicationThrottledReplicas []metav1.List `json:"followerReplicationThrottledReplicas"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	IndexIntervalBytes *int `json:"indexIntervalBytes"`
	// +optional
	LeaderReplicationThrottledReplicas []metav1.List `json:"leaderReplicationThrottledReplicas"`
	// +optional
	LocalRetentionBytes *int64 `json:"localRetentionBytes"`
	// +optional
	LocalRetentionMs *int64 `json:"localRetentionMs"`
	// +optional
	// +kubebuilder:validation:Minimum:=1
	MaxCompactionLagMs *int64 `json:"maxCompactionLagMs"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	MaxMessageBytes *int `json:"maxMessageBytes"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	MessageTimestampDifferenceMaxMs *int64 `json:"messageTimestampDifferenceMaxMs"`
	// +optional
	// +kubebuilder:validation:Enum:=CreateTime;LogAppendTime
	MessageTimestampType *string `json:"messageTimestampType"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	// +kubebuilder:validation:Maximum:=1
	MinCleanableDirtyRatio *int32 `json:"minCleanableDirtyRatio"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	MinCompactionLagMs *int64 `json:"minCompactionLagMs"`
	// +optional
	// +kubebuilder:validation:Minimum:=1
	MinInsyncReplicas *int `json:"minInsyncReplicas"`
	// +optional
	// +kubebuilder:validation:Enum:=true;false
	Preallocate *bool `json:"preallocate"`
	// +optional
	// +kubebuilder:validation:Enum:=true;false
	RemoteStorageEnable *bool `json:"remoteStorageEnable,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum:=-1
	RetentionBytes *int64 `json:"retentionBytes,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum:=-1
	RetentionMs *int64 `json:"retentionMs,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum:=14
	SegmentBytes *int `json:"segmentBytes,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	SegmentIndexBytes *int `json:"segmentIndexBytes,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum:=0
	SegmentJitterMs *int64 `json:"segmentJitterMs"`
	// +optional
	// +kubebuilder:validation:Minimum:=1
	SegmentMs *int64 `json:"segmentMs"`
	// +optional
	// +kubebuilder:validation:Enum:=true;false
	UncleanLeaderElectionEnable *bool `json:"uncleanLeaderElectionEnable"`
	// +optional
	// +kubebuilder:validation:Enum:=true;false
	MessageDownconversionEnable *bool `json:"messageDownconversionEnable"`
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
