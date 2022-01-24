package acl

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kadm"
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
func List(ctx context.Context, cl *kadm.Client, name string) (*kadm.DescribeACLsResults, error) {
//	b := *kadm.NewACLs().Topics("sample_topic")
	b := (*kadm.ACLBuilder).AllowHosts("*")
	resp, _ := cl.DescribeACLs(ctx, b)

	fmt.Println("*** LIST RESPONSE ***", resp)

	return &resp, nil
}

// Create creates an ACL from the Kafka side
func Create() {

}

// Delete deletes an ACL from the Kafka side
func Delete() {

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
