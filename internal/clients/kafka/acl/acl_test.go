package acl

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"k8s.io/apimachinery/pkg/util/json"

	"github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
)

// fakeACLAdmin is an in-process implementation of adminClient for unit tests.
type fakeACLAdmin struct {
	createResults kadm.CreateACLsResults
	createErr     error
	describeResults kadm.DescribeACLsResults
	describeErr   error
	deleteResults kadm.DeleteACLsResults
	deleteErr     error
}

func (f *fakeACLAdmin) CreateACLs(_ context.Context, _ *kadm.ACLBuilder) (kadm.CreateACLsResults, error) {
	return f.createResults, f.createErr
}

func (f *fakeACLAdmin) DescribeACLs(_ context.Context, _ *kadm.ACLBuilder) (kadm.DescribeACLsResults, error) {
	return f.describeResults, f.describeErr
}

func (f *fakeACLAdmin) DeleteACLs(_ context.Context, _ *kadm.ACLBuilder) (kadm.DeleteACLsResults, error) {
	return f.deleteResults, f.deleteErr
}

var dataTesting = []byte(os.Getenv("KAFKA_CONFIG"))

var baseACL = AccessControlList{
	ResourceName:              kafka.TestACLName,
	ResourceType:              kafka.ACLResourceTypeTopic,
	ResourcePrincipal:         kafka.TestACLPrincipal,
	ResourceHost:              "*",
	ResourceOperation:         kafka.ACLOperationAlterConfigs,
	ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
	ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
}

var baseJSONACL = `{
		"ResourceName": "acl1",
		"ResourceType": "Topic",
		"ResourcePrincipal": "User:Ken",
		"ResourceHost": "*",
		"ResourceOperation": "AlterConfigs",
		"ResourcePermissionType": "Allow",
		"ResourcePatternTypeFilter": "Literal"

		}`

func TestCompareAcls(t *testing.T) {
	type args struct {
		extname  AccessControlList
		observed AccessControlList
	}

	aclTesting := baseACL

	cases := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "PassCompare",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              kafka.TestACLName,
					ResourceType:              kafka.ACLResourceTypeTopic,
					ResourcePrincipal:         kafka.TestACLPrincipal,
					ResourceHost:              "*",
					ResourceOperation:         kafka.ACLOperationAlterConfigs,
					ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
					ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
				},
			},
			want: true,
		},
		{
			name: "FailAclName",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              "acl10",
					ResourceType:              kafka.ACLResourceTypeTopic,
					ResourcePrincipal:         kafka.TestACLPrincipal,
					ResourceHost:              "*",
					ResourceOperation:         kafka.ACLOperationAlterConfigs,
					ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
					ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
				},
			},
			want: false,
		},
		{
			name: "FailResourceType",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              kafka.TestACLName,
					ResourceType:              "Topical",
					ResourcePrincipal:         kafka.TestACLPrincipal,
					ResourceHost:              "*",
					ResourceOperation:         kafka.ACLOperationAlterConfigs,
					ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
					ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
				},
			},
			want: false,
		},
		{
			name: "FailPrinciple",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              kafka.TestACLName,
					ResourceType:              kafka.ACLResourceTypeTopic,
					ResourcePrincipal:         "User:NotKen",
					ResourceHost:              "*",
					ResourceOperation:         kafka.ACLOperationAlterConfigs,
					ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
					ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
				},
			},
			want: false,
		},
		{
			name: "FailSpecificHost",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              kafka.TestACLName,
					ResourceType:              kafka.ACLResourceTypeTopic,
					ResourcePrincipal:         kafka.TestACLPrincipal,
					ResourceHost:              "acme.com",
					ResourceOperation:         kafka.ACLOperationAlterConfigs,
					ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
					ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
				},
			},
			want: false,
		},
		{
			name: "FailOperationName",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              kafka.TestACLName,
					ResourceType:              kafka.ACLResourceTypeTopic,
					ResourcePrincipal:         kafka.TestACLPrincipal,
					ResourceHost:              "*",
					ResourceOperation:         kafka.ACLOperationRead,
					ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
					ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
				},
			},
			want: false,
		},
		{
			name: "FailPermissionType",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              kafka.TestACLName,
					ResourceType:              kafka.ACLResourceTypeTopic,
					ResourcePrincipal:         kafka.TestACLPrincipal,
					ResourceHost:              "*",
					ResourceOperation:         kafka.ACLOperationAlterConfigs,
					ResourcePermissionType:    kafka.ACLOperationWrite,
					ResourcePatternTypeFilter: "Any",
				},
			},
			want: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareAcls(tt.args.extname, tt.args.observed)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("CompareAcls() = -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConvertFromJSON(t *testing.T) {
	type args struct {
		extname string
	}

	baseACL := baseACL

	cases := []struct {
		name    string
		args    args
		want    *AccessControlList
		wantErr bool
	}{
		{
			name: "InvalidACL",
			args: args{
				extname: "acl1",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "ValidACL",
			args: args{
				extname: baseJSONACL,
			},
			want:    &baseACL,
			wantErr: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertFromJSON(tt.args.extname)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertFromJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("ConvertFromJSON() -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConvertToJSON(t *testing.T) {
	type args struct {
		acl *AccessControlList
	}

	aclJSONMarshal, _ := json.Marshal(baseACL)
	aclJSONString := string(aclJSONMarshal)

	cases := map[string]struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		"PassJsonMarshal": {
			name: "SuccessfulMarshal",
			args: args{
				acl: &baseACL,
			},
			want:    aclJSONString,
			wantErr: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertToJSON(tt.args.acl)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertToJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ConvertToJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreate(t *testing.T) {
	if len(dataTesting) == 0 {
		t.Skip("KAFKA_CONFIG not set, skipping integration test")
	}

	ctx := context.Background()
	newAc, err := kafka.NewAdminClient(ctx, dataTesting, nil)
	if err != nil {
		t.Fatalf("failed to create admin client: %v", err)
	}

	testACL := &AccessControlList{
		ResourceName:              "test-acl-create-topic",
		ResourceType:              kafka.ACLResourceTypeTopic,
		ResourcePrincipal:         "User:user",
		ResourceHost:              "*",
		ResourceOperation:         kafka.ACLOperationRead,
		ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
		ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
	}

	err = Create(ctx, newAc, testACL)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Clean up
	t.Cleanup(func() {
		_ = Delete(ctx, newAc, testACL)
	})
}

func TestDelete(t *testing.T) {
	if len(dataTesting) == 0 {
		t.Skip("KAFKA_CONFIG not set, skipping integration test")
	}

	ctx := context.Background()
	newAc, err := kafka.NewAdminClient(ctx, dataTesting, nil)
	if err != nil {
		t.Fatalf("failed to create admin client: %v", err)
	}

	testACL := &AccessControlList{
		ResourceName:              "test-acl-delete-topic",
		ResourceType:              kafka.ACLResourceTypeTopic,
		ResourcePrincipal:         "User:user",
		ResourceHost:              "*",
		ResourceOperation:         kafka.ACLOperationWrite,
		ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
		ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
	}

	// Create first, then delete
	err = Create(ctx, newAc, testACL)
	if err != nil {
		t.Fatalf("Create() setup error = %v", err)
	}

	err = Delete(ctx, newAc, testACL)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestGenerate(t *testing.T) {
	params := &v1alpha1.AccessControlListParameters{
		ResourceName:              "my-topic",
		ResourceType:              kafka.ACLResourceTypeTopic,
		ResourcePrincipal:         "User:alice",
		ResourceHost:              "*",
		ResourceOperation:         kafka.ACLOperationRead,
		ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
		ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
	}

	got := Generate(params)
	want := &AccessControlList{
		ResourceName:              "my-topic",
		ResourceType:              kafka.ACLResourceTypeTopic,
		ResourcePrincipal:         "User:alice",
		ResourceHost:              "*",
		ResourceOperation:         kafka.ACLOperationRead,
		ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
		ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Generate() = %v, want %v", got, want)
	}
}

func TestIsUpToDate(t *testing.T) {
	cases := []struct {
		name string
		in   *v1alpha1.AccessControlListParameters
		obs  *AccessControlList
		want bool
	}{
		{
			name: "UpToDate",
			in: &v1alpha1.AccessControlListParameters{
				ResourceName:              kafka.TestACLName,
				ResourceType:              kafka.ACLResourceTypeTopic,
				ResourcePrincipal:         kafka.TestACLPrincipal,
				ResourceHost:              "*",
				ResourceOperation:         kafka.ACLOperationAlterConfigs,
				ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
				ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
			},
			obs: &AccessControlList{
				ResourceName:              kafka.TestACLName,
				ResourceType:              kafka.ACLResourceTypeTopic,
				ResourcePrincipal:         kafka.TestACLPrincipal,
				ResourceHost:              "*",
				ResourceOperation:         kafka.ACLOperationAlterConfigs,
				ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
				ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
			},
			want: true,
		},
		{
			name: "DiffOperation",
			in: &v1alpha1.AccessControlListParameters{
				ResourceName:              kafka.TestACLName,
				ResourceType:              kafka.ACLResourceTypeTopic,
				ResourcePrincipal:         kafka.TestACLPrincipal,
				ResourceHost:              "*",
				ResourceOperation:         kafka.ACLOperationRead,
				ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
				ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
			},
			obs: &AccessControlList{
				ResourceName:              kafka.TestACLName,
				ResourceType:              kafka.ACLResourceTypeTopic,
				ResourcePrincipal:         kafka.TestACLPrincipal,
				ResourceHost:              "*",
				ResourceOperation:         kafka.ACLOperationAlterConfigs,
				ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
				ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
			},
			want: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUpToDate(tt.in, tt.obs); got != tt.want {
				t.Errorf("IsUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestList(t *testing.T) {
	if len(dataTesting) == 0 {
		t.Skip("KAFKA_CONFIG not set, skipping integration test")
	}

	ctx := context.Background()
	newAc, err := kafka.NewAdminClient(ctx, dataTesting, nil)
	if err != nil {
		t.Fatalf("failed to create admin client: %v", err)
	}

	testACL := &AccessControlList{
		ResourceName:              "test-acl-list-topic",
		ResourceType:              kafka.ACLResourceTypeTopic,
		ResourcePrincipal:         "User:user",
		ResourceHost:              "*",
		ResourceOperation:         kafka.ACLOperationDescribe,
		ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
		ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
	}

	// Create the ACL first
	err = Create(ctx, newAc, testACL)
	if err != nil {
		t.Fatalf("Create() setup error = %v", err)
	}
	t.Cleanup(func() {
		_ = Delete(ctx, newAc, testACL)
	})

	// ACLs may not be immediately visible after creation; retry briefly.
	var got *AccessControlList
	for range 10 {
		got, err = List(ctx, newAc, testACL)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if got != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if got == nil {
		t.Fatal("List() returned nil, expected ACL")
	}

	// Verify all fields that feed into status.atProvider
	if got.ResourceType != testACL.ResourceType {
		t.Errorf("ResourceType = %q, want %q", got.ResourceType, testACL.ResourceType)
	}
	if got.ResourcePrincipal != testACL.ResourcePrincipal {
		t.Errorf("ResourcePrincipal = %q, want %q", got.ResourcePrincipal, testACL.ResourcePrincipal)
	}
	if got.ResourceHost != testACL.ResourceHost {
		t.Errorf("ResourceHost = %q, want %q", got.ResourceHost, testACL.ResourceHost)
	}
	if got.ResourceOperation != testACL.ResourceOperation {
		t.Errorf("ResourceOperation = %q, want %q", got.ResourceOperation, testACL.ResourceOperation)
	}
	if got.ResourcePermissionType != testACL.ResourcePermissionType {
		t.Errorf("ResourcePermissionType = %q, want %q", got.ResourcePermissionType, testACL.ResourcePermissionType)
	}
	if got.ResourcePatternTypeFilter != testACL.ResourcePatternTypeFilter {
		t.Errorf("ResourcePatternTypeFilter = %q, want %q", got.ResourcePatternTypeFilter, testACL.ResourcePatternTypeFilter)
	}
}

// --- Unit tests for broker-level error propagation (no real Kafka needed) ---

func TestCreateBrokerError(t *testing.T) {
	cl := &fakeACLAdmin{
		createResults: kadm.CreateACLsResults{
			{Principal: "User:alice", Err: kerr.ClusterAuthorizationFailed},
		},
	}
	err := Create(context.Background(), cl, &baseACL)
	if err == nil {
		t.Fatal("Create() expected error for broker-level Err, got nil")
	}
}

func TestCreateEmptyResponse(t *testing.T) {
	cl := &fakeACLAdmin{createResults: kadm.CreateACLsResults{}}
	err := Create(context.Background(), cl, &baseACL)
	if err == nil {
		t.Fatal("Create() expected error for empty response, got nil")
	}
}

func TestListBrokerError(t *testing.T) {
	cl := &fakeACLAdmin{
		describeResults: kadm.DescribeACLsResults{
			{Err: kerr.ClusterAuthorizationFailed},
		},
	}
	got, err := List(context.Background(), cl, &baseACL)
	if err == nil {
		t.Fatal("List() expected error for broker-level Err, got nil")
	}
	if got != nil {
		t.Errorf("List() expected nil result on error, got %v", got)
	}
}

func TestListNotFound(t *testing.T) {
	cl := &fakeACLAdmin{
		describeResults: kadm.DescribeACLsResults{
			{Described: kadm.DescribedACLs{}},
		},
	}
	got, err := List(context.Background(), cl, &baseACL)
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("List() expected nil for no matching ACL, got %v", got)
	}
}

func TestListEmptyResponse(t *testing.T) {
	cl := &fakeACLAdmin{describeResults: kadm.DescribeACLsResults{}}
	got, err := List(context.Background(), cl, &baseACL)
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("List() expected nil for empty response, got %v", got)
	}
}

func TestDeleteBrokerError(t *testing.T) {
	cl := &fakeACLAdmin{
		deleteResults: kadm.DeleteACLsResults{
			{Err: kerr.ClusterAuthorizationFailed},
		},
	}
	err := Delete(context.Background(), cl, &baseACL)
	if err == nil {
		t.Fatal("Delete() expected error for broker-level Err, got nil")
	}
}

// TestListAtProviderNotFound verifies that List returns nil when the ACL does not exist.
func TestListAtProviderNotFound(t *testing.T) {
	if len(dataTesting) == 0 {
		t.Skip("KAFKA_CONFIG not set, skipping integration test")
	}

	ctx := context.Background()
	newAc, err := kafka.NewAdminClient(ctx, dataTesting, nil)
	if err != nil {
		t.Fatalf("failed to create admin client: %v", err)
	}

	nonExistentACL := &AccessControlList{
		ResourceName:              "non-existent-acl-topic",
		ResourceType:              kafka.ACLResourceTypeTopic,
		ResourcePrincipal:         "User:nobody",
		ResourceHost:              "*",
		ResourceOperation:         kafka.ACLOperationRead,
		ResourcePermissionType:    kafka.ACLPermissionTypeAllow,
		ResourcePatternTypeFilter: kafka.ACLPatternTypeLiteral,
	}

	got, err := List(ctx, newAc, nonExistentACL)
	if err != nil {
		t.Fatalf("List() error = %v, expected nil result without error", err)
	}
	if got != nil {
		t.Errorf("List() = %v, expected nil for non-existent ACL", got)
	}
}
