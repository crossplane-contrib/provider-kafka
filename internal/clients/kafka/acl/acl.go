package acl

import (
	"context"
	"encoding/json"
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
	Name                      string `json:"Name"`
	ResourceType              string `json:"ResourceType"`
	Principle                 string `json:"Principle"`
	Host                      string `json:"Host"`
	Operation                 string `json:"Operation"`
	PermissionType            string `json:"PermissionType"`
	ResourcePatternTypeFilter string `json:"ResourcePatternTypeFilter"`
}


// List lists all the ACLs in Kafka
func List(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) (*AccessControlList, error) {

	o, err := kmsg.ParseACLOperation(strings.ToLower(accessControlList.Operation))
	if err != nil {
		return nil, errors.Wrap(err, "did not return ACL Operation")
	}

	ao := []kadm.ACLOperation{o}

	rpt, err := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))
	if err != nil {
		return nil, errors.Wrap(err, "did not return parsing of ACL pattern")
	}

	b := kadm.ACLBuilder{}
	ab := b.Topics(accessControlList.Name).Allow(accessControlList.Principle).AllowHosts(accessControlList.Host).Operations(ao[0]).ResourcePatternType(rpt)

	resp, err := cl.DescribeACLs(ctx, ab)

	if resp == nil {
		return nil, errors.Wrap(err, "describe ACLs response is empty")
	}

	exists := resp[0].Described

	if exists == nil {
		return nil, errors.Wrap(err, "cannot describe ACL, it does not exist")
	}

	acl := AccessControlList{}
	acl.ResourceType = accessControlList.ResourceType
	acl.Principle = accessControlList.Principle
	acl.Host = accessControlList.Host
	acl.Operation = accessControlList.Operation
	acl.PermissionType = accessControlList.PermissionType
	acl.ResourcePatternTypeFilter = accessControlList.ResourcePatternTypeFilter

	return &acl, nil
}

// Create creates an ACL from the Kafka side
func Create(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) error {

	o, _ := kmsg.ParseACLOperation(strings.ToLower(accessControlList.Operation))
	ao := []kadm.ACLOperation{o}

	rpt, _ := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))

	b := kadm.ACLBuilder{}
	ab := b.Allow(accessControlList.Principle).AllowHosts(accessControlList.Host).Operations(ao[0]).ResourcePatternType(rpt)

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

	o, _ := kmsg.ParseACLOperation(strings.ToLower(accessControlList.Operation))
	ao := []kadm.ACLOperation{o}

	rpt, _ := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))

	b := kadm.ACLBuilder{}
	ab := b.Topics(accessControlList.Name).Allow(accessControlList.Principle).AllowHosts(accessControlList.Host).Operations(ao[0]).ResourcePatternType(rpt)

	resp, err := cl.DeleteACLs(ctx, ab)
	if err != nil {
		return err
	}

	fmt.Println("Delete Response:", resp)

	return nil
}

func ConvertToJson(acl *AccessControlList) (string, error) {
	j, err := json.Marshal(acl)
	if err != nil{
		return "", errors.Wrap(err, "describe ACLs response is empty")
	}
	name := string(j)

	return name, nil
}

func ConvertFromJson(extname string) (*AccessControlList, error) {
	acl := AccessControlList{}
	err := json.Unmarshal([]byte(extname), &acl)
	if err != nil{
		return nil, errors.Wrap(err, "describe ACLs response is empty")
	}
	return &acl, nil
}

func CompareAcls(extname AccessControlList, observed AccessControlList) bool {

	fmt.Println("Extname: ", extname)
	fmt.Println("Observed: ", observed)

	if extname == observed {
		return true
	}
	return false
}

// Generate is used to convert Crossplane AccessControlListParameters to Kafka's AccessControlList.
func Generate(name string, params *v1alpha1.AccessControlListParameters) *AccessControlList {
	acl := &AccessControlList{
		Name:                      name,
		ResourceType:              params.ResourceType,
		Principle:                 params.Principle,
		Host:                      params.Host,
		Operation:                 params.Operation,
		PermissionType:            params.PermissionType,
		ResourcePatternTypeFilter: params.ResourcePatternTypeFilter,
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
	if in.Principle != observed.Principle {
		return false
	}
	if in.Host != observed.Host {
		return false
	}
	if in.Operation != observed.Operation {
		return false
	}
	if in.PermissionType != observed.PermissionType {
		return false
	}
	if in.ResourcePatternTypeFilter != observed.ResourcePatternTypeFilter {
		return false
	}
	return true
}