package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Package type metadata.
const (
	Group   = "kafka.m.crossplane.io"
	Version = "v1alpha1"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		metav1.AddToGroupVersion(s, SchemeGroupVersion)
		return nil
	})
)

// ProviderConfig type metadata.
var (
	ProviderConfigKind             = reflect.TypeOf(ProviderConfig{}).Name()
	ProviderConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ProviderConfigKind}.String()
	ProviderConfigGroupVersionKind = SchemeGroupVersion.WithKind(ProviderConfigKind)
)

// ProviderConfigUsage type metadata.
var (
	ProviderConfigUsageKind             = reflect.TypeOf(ProviderConfigUsage{}).Name()
	ProviderConfigUsageGroupVersionKind = SchemeGroupVersion.WithKind(ProviderConfigUsageKind)

	ProviderConfigUsageListKind             = reflect.TypeOf(ProviderConfigUsageList{}).Name()
	ProviderConfigUsageListGroupVersionKind = SchemeGroupVersion.WithKind(ProviderConfigUsageListKind)
)

// ClusterProviderConfig type metadata
var (
	ClusterProviderConfigKind             = reflect.TypeOf(ClusterProviderConfig{}).Name()
	ClusterProviderConfigGroupKind        = schema.GroupKind{Group: Group, Kind: ClusterProviderConfigKind}.String()
	ClusterProviderConfigGroupVersionKind = SchemeGroupVersion.WithKind(ClusterProviderConfigKind)
)

// ClusterProviderConfigUsage type metadata.
var (
	ClusterProviderConfigUsageKind             = reflect.TypeOf(ClusterProviderConfigUsage{}).Name()
	ClusterProviderConfigUsageGroupVersionKind = SchemeGroupVersion.WithKind(ClusterProviderConfigUsageKind)

	ClusterProviderConfigUsageListKind             = reflect.TypeOf(ClusterProviderConfigUsageList{}).Name()
	ClusterProviderConfigUsageListGroupVersionKind = SchemeGroupVersion.WithKind(ClusterProviderConfigUsageListKind)
)

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion,
			&ProviderConfig{}, &ProviderConfigList{},
			&ProviderConfigUsage{}, &ProviderConfigUsageList{},
			&ClusterProviderConfig{}, &ClusterProviderConfigList{},
			&ClusterProviderConfigUsage{}, &ClusterProviderConfigUsageList{},
		)
		return nil
	})
}
