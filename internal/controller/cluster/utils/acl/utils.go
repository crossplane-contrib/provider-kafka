package acl

import (
	"github.com/crossplane-contrib/provider-kafka/apis/cluster/acl/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/acl"
)

// Generate is used to convert Crossplane AccessControlListParameters to Kafka's AccessControlList.
func Generate(params *v1alpha1.AccessControlListParameters) *acl.AccessControlList {
	_acl := &acl.AccessControlList{
		ResourceName:              params.ResourceName,
		ResourceType:              params.ResourceType,
		ResourcePrincipal:         params.ResourcePrincipal,
		ResourceHost:              params.ResourceHost,
		ResourceOperation:         params.ResourceOperation,
		ResourcePermissionType:    params.ResourcePermissionType,
		ResourcePatternTypeFilter: params.ResourcePatternTypeFilter,
	}

	return _acl
}

// IsUpToDate returns true if the supplied Kubernetes resource differs from the
// supplied Kafka ACLs.
func IsUpToDate(in *v1alpha1.AccessControlListParameters, observed *acl.AccessControlList) bool {

	if in.ResourceType != observed.ResourceType {
		return false
	}
	if in.ResourcePrincipal != observed.ResourcePrincipal {
		return false
	}
	if in.ResourceHost != observed.ResourceHost {
		return false
	}
	if in.ResourceOperation != observed.ResourceOperation {
		return false
	}
	if in.ResourcePermissionType != observed.ResourcePermissionType {
		return false
	}
	if in.ResourcePatternTypeFilter != observed.ResourcePatternTypeFilter {
		return false
	}
	return true
}
