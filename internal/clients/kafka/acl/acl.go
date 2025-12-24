package acl

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// AccessControlList is a holistic representation of a Kafka ACL with configurable
// fields
type AccessControlList struct {
	ResourceName              string `json:"ResourceName"`
	ResourceType              string `json:"ResourceType"`
	ResourcePrincipal         string `json:"ResourcePrincipal"`
	ResourceHost              string `json:"ResourceHost"`
	ResourceOperation         string `json:"ResourceOperation"`
	ResourcePermissionType    string `json:"ResourcePermissionType"`
	ResourcePatternTypeFilter string `json:"ResourcePatternTypeFilter"`
}

// List lists all the ACLs in Kafka
func List(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) (*AccessControlList, error) { //nolint:gocyclo

	o, err := kmsg.ParseACLOperation(strings.ToLower(accessControlList.ResourceOperation))
	if err != nil {
		return nil, errors.Wrap(err, "did not return ACL Operation")
	}

	ao := []kadm.ACLOperation{o}

	rpt, err := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))
	if err != nil {
		return nil, errors.Wrap(err, "did not return parsing of ACL pattern")
	}

	b := kadm.ACLBuilder{}
	ab := b.Allow(accessControlList.ResourcePrincipal).AllowHosts(accessControlList.ResourceHost).Operations(ao[0]).ResourcePatternType(rpt)

	switch accessControlList.ResourceType {
	case "Topic":
		ab = ab.Topics(accessControlList.ResourceName)
	case "Group":
		ab = ab.Groups(accessControlList.ResourceName)
	case "TransactionalID":
		ab = ab.TransactionalIDs(accessControlList.ResourceName)
	case "Cluster":
		ab = ab.Clusters()
	case "Any":
		ab = ab.AnyResource(accessControlList.ResourceName)
	}

	resp, err := cl.DescribeACLs(ctx, ab)
	if err != nil {
		return nil, errors.Wrap(err, "describe ACLs response is empty")
	}
	if len(resp) == 0 || len(resp[0].Described) == 0 {
		return nil, nil
	}

	acl := AccessControlList{}
	acl.ResourceType = accessControlList.ResourceType
	acl.ResourcePrincipal = accessControlList.ResourcePrincipal
	acl.ResourceHost = accessControlList.ResourceHost
	acl.ResourceOperation = accessControlList.ResourceOperation
	acl.ResourcePermissionType = accessControlList.ResourcePermissionType
	acl.ResourcePatternTypeFilter = accessControlList.ResourcePatternTypeFilter

	return &acl, nil
}

// Create creates an ACL from the Kafka side
func Create(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) error {

	o, _ := kmsg.ParseACLOperation(strings.ToLower(accessControlList.ResourceOperation))
	ao := []kadm.ACLOperation{o}

	rpt, _ := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))

	b := kadm.ACLBuilder{}
	ab := b.Allow(accessControlList.ResourcePrincipal).AllowHosts(accessControlList.ResourceHost).Operations(ao[0]).ResourcePatternType(rpt)

	switch accessControlList.ResourceType {
	case "Topic":
		ab = ab.Topics(accessControlList.ResourceName)
	case "Group":
		ab = ab.Groups(accessControlList.ResourceName)
	case "TransactionalID":
		ab = ab.TransactionalIDs(accessControlList.ResourceName)
	case "Cluster":
		ab = ab.Clusters()
	case "Any":
		ab = ab.AnyResource(accessControlList.ResourceName)
	}

	resp, err := cl.CreateACLs(ctx, ab)
	if err != nil {
		return err
	}
	if resp != nil {
		a := resp[0].Principal
		if len(a) == 0 {
			return errors.New("no create response for acl")
		}
	}

	return nil
}

// Delete creates an ACL from the Kafka side
func Delete(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) error {

	o, _ := kmsg.ParseACLOperation(strings.ToLower(accessControlList.ResourceOperation))
	ao := []kadm.ACLOperation{o}

	rpt, _ := kmsg.ParseACLResourcePatternType(strings.ToLower(accessControlList.ResourcePatternTypeFilter))

	b := kadm.ACLBuilder{}
	ab := b.Allow(accessControlList.ResourcePrincipal).AllowHosts(accessControlList.ResourceHost).Operations(ao[0]).ResourcePatternType(rpt)

	switch accessControlList.ResourceType {
	case "Topic":
		ab = ab.Topics(accessControlList.ResourceName)
	case "Group":
		ab = ab.Groups(accessControlList.ResourceName)
	case "TransactionalID":
		ab = ab.TransactionalIDs(accessControlList.ResourceName)
	case "Cluster":
		ab = ab.Clusters()
	case "Any":
		ab = ab.AnyResource(accessControlList.ResourceName)
	}

	_, err := cl.DeleteACLs(ctx, ab)
	return err
}

// ConvertToJSON performs a json marshalling for ACLs
func ConvertToJSON(acl *AccessControlList) (string, error) {
	j, err := json.Marshal(acl)
	if err != nil {
		return "", errors.Wrap(err, "describe ACLs response is empty")
	}
	name := string(j)

	return name, nil
}

// ConvertFromJSON performs a json unmarshalling for ACLs
func ConvertFromJSON(extname string) (*AccessControlList, error) {
	acl := AccessControlList{}
	err := json.Unmarshal([]byte(extname), &acl)
	if err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal external-name JSON")
	}
	return &acl, nil
}

// Diff performs a Diff of and existing and observed ACL and provides
// the User with an error string and the difference if one is found
func Diff(existing AccessControlList, observed AccessControlList) []string {
	diff := make([]string, 0)
	if existing.ResourceType != observed.ResourceType {
		str := "Resource Type has been updated, which is not allowed."
		diff = append(diff, str)
	}
	if existing.ResourcePrincipal != observed.ResourcePrincipal {
		str := "Resource Principal has been updated, which is not allowed."
		diff = append(diff, str)
	}
	if existing.ResourceHost != observed.ResourceHost {
		str := "Resource Host has been updated, which is not allowed."
		diff = append(diff, str)
	}
	if existing.ResourceOperation != observed.ResourceOperation {
		str := "Resource Operation has been updated, which is not allowed."
		diff = append(diff, str)
	}
	if existing.ResourcePermissionType != observed.ResourcePermissionType {
		str := "Resource Permission Type has been updated, which is not allowed."
		diff = append(diff, str)
	}
	if existing.ResourcePatternTypeFilter != observed.ResourcePatternTypeFilter {
		str := "Resource Pattern Type Filter has been updated, which is not allowed."
		diff = append(diff, str)
	}
	return diff
}

// CompareAcls performs an observed to incoming ACL comparison
func CompareAcls(extname AccessControlList, observed AccessControlList) bool {
	return extname == observed
}
