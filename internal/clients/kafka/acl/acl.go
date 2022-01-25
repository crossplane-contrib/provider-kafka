package acl

import (
	"context"

	"github.com/pkg/errors"

	"github.com/crossplane-contrib/provider-kafka/apis/acl/v1alpha1"
	"github.com/twmb/franz-go/pkg/kadm"
	// "github.com/twmb/franz-go/pkg/kmsg"
)

// AccessControlList is a holistic representation of a Kafka ACL with configurable
// fields
type AccessControlList struct {
	Name                            string
	ResourceType                    string
	AccessControlListPrinciple      string
	AccessControlListHost           string
	AccessControlListOperation      string
	AccessControlListPermissionType string
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
func List() {

}

// Create creates an ACL from the Kafka side
func Create(ctx context.Context, cl *kadm.Client, accessControlList *AccessControlList) error {

	b := kadm.ACLBuilder{}
	// ab := b.Topics(accessControlList.Name).Allow(accessControlList.AccessControlListPrinciple).AllowHosts(accessControlList.AccessControlListHost).Operations(accessControlList.AccessControlListOperation).ResourcePatternType(accessControlList.ResourcePatternTypeFilter)
	ab := b.Topics(accessControlList.Name).Allow(accessControlList.AccessControlListPrinciple).AllowHosts(accessControlList.AccessControlListHost).Operations(3).ResourcePatternType(1)

	resp, err := cl.CreateACLs(ctx, ab)
	if err != nil {
		return err
	}

	a, ok := resp[accessControlList.Name]
	if !ok {
		return errors.New("no create response for acl")
	}
	if a.Err != nil {
		return errors.Wrap(a.Err, "cannot create acl")
	}

	return nil
}

// Delete deletes an ACL from the Kafka side
func Delete() {

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
func IsUpToDate() {

}

func stringValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
