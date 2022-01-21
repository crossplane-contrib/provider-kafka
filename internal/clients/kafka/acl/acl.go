package acl

// AccessControlList is a holistic representation of a Kafka ACL with configurable fields
type AccessControlList struct {
	Name                            string
	ResourceType                    string
	AccessControlListPrinciple      string
	AccessControlListHost           string
	AccessControlListOperation      string
	AccessControlListPermissionType string
	ResourcePatternTypeFilter       string
}

// List gets the ACL List from the Kafka side and returns all ACLs
func List() {

}

// Create creates an ACL from the Kafka side
func Create() {

}

// Delete deletes an ACL from the Kafka side
func Delete() {

}

// LateInitializeSpec fills empty ACL spec fields with the data retrieved from Kafka.
func LateInitializeSpec(){

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
