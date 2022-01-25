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
func List() {

}

// Create creates an ACL from the Kafka side
func Create(ctx context.Context, cl *kadm.Client, name string) (*kadm.CreateACLsResults, error) {

	b := kadm.ACLBuilder{}
	ab := b.Topics("sample_topic").Allow("User:Jon").AllowHosts("*").Operations(kadm.OpWrite).ResourcePatternType(kadm.ACLPatternLiteral)

	c, err := cl.CreateACLs(ctx, ab)

	fmt.Println("*** CREATING ACL ***", c)

	fmt.Println("ERROR: ", err)

	fmt.Println("Underlying ERROR: ", c[0].Err)

	return &c, err
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
