package acl

import (
	"context"
	"fmt"
	"strings"

	"github.com/crossplane-contrib/provider-kafka/apis/acl/v1alpha1"

	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// AccessControlList is a holistic representation of a Kafka ACL with configurable
// fields
type AccessControlList struct {
	Name                            string
	ResourceType                    string
	Principle      string
	Host           string
	Operation      string
	PermissionType string
	ResourcePatternTypeFilter       string
}

// Topic is a holistic representation of a Kafka Topic with all configurable
// fields
type Topic struct {
	Name              string
	ReplicationFactor int16
	Partitions        int32
	ID                string
	Config            map[string]*string
}

// List lists all the ACLs in Kafka
func List(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) (*AccessControlList, error) {

	o, _ := kmsg.ParseACLOperation(strings.ToLower(accessControlList.AccessControlListOperation))
	ao := []kadm.ACLOperation{o}

	rpt, _ := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))

	b := kadm.ACLBuilder{}
	ab := b.Topics(accessControlList.Name).Allow(accessControlList.AccessControlListPrinciple).AllowHosts(accessControlList.AccessControlListHost).Operations(ao[0]).ResourcePatternType(rpt)

	resp, err := cl.DescribeACLs(ctx, ab)
	exists := resp[0].Described

	if exists == nil {
		return nil, errors.Wrap(err, "cannot describe ACL, it does not exist")
	}

	acl := AccessControlList{}
	acl.ResourceType = accessControlList.ResourceType
	acl.AccessControlListPrinciple = accessControlList.AccessControlListPrinciple
	acl.AccessControlListHost = accessControlList.AccessControlListHost
	acl.AccessControlListOperation = accessControlList.AccessControlListOperation
	acl.AccessControlListPermissionType = accessControlList.AccessControlListPermissionType
	acl.ResourcePatternTypeFilter = accessControlList.ResourcePatternTypeFilter

	return &acl, nil
}

// Create creates an ACL from the Kafka side
func Create(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) error {

	o, _ := kmsg.ParseACLOperation(strings.ToLower(accessControlList.AccessControlListOperation))
	ao := []kadm.ACLOperation{o}

	rpt, _ := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))

	b := kadm.ACLBuilder{}
	ab := b.Allow(accessControlList.AccessControlListPrinciple).AllowHosts(accessControlList.AccessControlListHost).Operations(ao[0]).ResourcePatternType(rpt)

	switch accessControlList.ResourceType {
	case "Topic":
		ab = ab.Topics(accessControlList.Name)
	case "Group":
		ab = ab.Groups(accessControlList.Name)
	case "TransactionalID":
		ab = ab.TransactionalIDs(accessControlList.Name)
	}

	resp, err := cl.CreateACLs(ctx, ab)
	if err != nil {
		return err
	}

	a := resp[0].Principal
	if len(a) == 0 {
		return errors.New("no create response for acl")
	}

	return nil
}

// Delete creates an ACL from the Kafka side
func Delete(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) error {

	o, _ := kmsg.ParseACLOperation(strings.ToLower(accessControlList.AccessControlListOperation))
	ao := []kadm.ACLOperation{o}

	rpt, _ := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))

	b := kadm.ACLBuilder{}
	ab := b.Topics(accessControlList.Name).Allow(accessControlList.AccessControlListPrinciple).AllowHosts(accessControlList.AccessControlListHost).Operations(ao[0]).ResourcePatternType(rpt)

	resp, err := cl.DeleteACLs(ctx, ab)
	if err != nil {
		return err
	}

	fmt.Println("Delete Response:", resp)

	return nil
}

// Generate is used to convert Crossplane AccessControlListParameters to Kafka's AccessControlList.
func Generate(name string, params *v1alpha1.AccessControlListParameters) *AccessControlList {
	acl := &AccessControlList{
		Name:                            name,
		ResourceType:                    params.ResourceType,
		AccessControlListPrinciple:      params.AccessControlListPrinciple,
		AccessControlListHost:           params.AccessControlListHost,
		AccessControlListOperation:      params.AccessControlListOperation,
		AccessControlListPermissionType: params.AccessControlListPermissionType,
		ResourcePatternTypeFilter:       params.ResourcePatternTypeFilter,
	}

	return acl
}

// LateInitializeSpec fills empty ACL spec fields with the data retrieved from Kafka.
func LateInitializeSpec() bool {
	lateInitialized := true

	return lateInitialized
}

// IsUpToDate returns true if the supplied Kubernetes resource differs from the
// supplied Kafka ACLs.
func IsUpToDate(in *v1alpha1.AccessControlListParameters, observed *AccessControlList) bool {

	if in.ResourceType != observed.ResourceType {
		return false
	}
	if in.AccessControlListPrinciple != observed.AccessControlListPrinciple {
		return false
	}
	if in.AccessControlListPrinciple != observed.AccessControlListPrinciple {
		return false
	}
	if in.AccessControlListOperation != observed.AccessControlListOperation {
		return false
	}
	if in.AccessControlListPermissionType != observed.AccessControlListPermissionType {
		return false
	}
	if in.ResourcePatternTypeFilter != observed.ResourcePatternTypeFilter {
		return false
	}
	return true
}
