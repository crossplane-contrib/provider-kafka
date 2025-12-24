package acl

import (
	"context"
	"reflect"
	"testing"

	"github.com/crossplane-contrib/provider-kafka/apis/cluster/acl/v1alpha1"

	"github.com/google/go-cmp/cmp"
	"github.com/twmb/franz-go/pkg/kadm"

	"k8s.io/apimachinery/pkg/util/json"
)

var baseACL = AccessControlList{
	ResourceName:              "acl1",
	ResourceType:              "Topic",
	ResourcePrincipal:         "User:Ken",
	ResourceHost:              "*",
	ResourceOperation:         "AlterConfigs",
	ResourcePermissionType:    "Allow",
	ResourcePatternTypeFilter: "Literal",
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
					ResourceName:              "acl1",
					ResourceType:              "Topic",
					ResourcePrincipal:         "User:Ken",
					ResourceHost:              "*",
					ResourceOperation:         "AlterConfigs",
					ResourcePermissionType:    "Allow",
					ResourcePatternTypeFilter: "Literal",
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
					ResourceType:              "Topic",
					ResourcePrincipal:         "User:Ken",
					ResourceHost:              "*",
					ResourceOperation:         "AlterConfigs",
					ResourcePermissionType:    "Allow",
					ResourcePatternTypeFilter: "Literal",
				},
			},
			want: false,
		},
		{
			name: "FailResourceType",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              "acl1",
					ResourceType:              "Topical",
					ResourcePrincipal:         "User:Ken",
					ResourceHost:              "*",
					ResourceOperation:         "AlterConfigs",
					ResourcePermissionType:    "Allow",
					ResourcePatternTypeFilter: "Literal",
				},
			},
			want: false,
		},
		{
			name: "FailPrinciple",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              "acl1",
					ResourceType:              "Topic",
					ResourcePrincipal:         "User:NotKen",
					ResourceHost:              "*",
					ResourceOperation:         "AlterConfigs",
					ResourcePermissionType:    "Allow",
					ResourcePatternTypeFilter: "Literal",
				},
			},
			want: false,
		},
		{
			name: "FailSpecificHost",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              "acl1",
					ResourceType:              "Topic",
					ResourcePrincipal:         "User:Ken",
					ResourceHost:              "acme.com",
					ResourceOperation:         "AlterConfigs",
					ResourcePermissionType:    "Allow",
					ResourcePatternTypeFilter: "Literal",
				},
			},
			want: false,
		},
		{
			name: "FailOperationName",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              "acl1",
					ResourceType:              "Topic",
					ResourcePrincipal:         "User:Ken",
					ResourceHost:              "*",
					ResourceOperation:         "Read",
					ResourcePermissionType:    "Allow",
					ResourcePatternTypeFilter: "Literal",
				},
			},
			want: false,
		},
		{
			name: "FailPermissionType",
			args: args{
				extname: aclTesting,
				observed: AccessControlList{
					ResourceName:              "acl1",
					ResourceType:              "Topic",
					ResourcePrincipal:         "User:Ken",
					ResourceHost:              "*",
					ResourceOperation:         "AlterConfigs",
					ResourcePermissionType:    "Write",
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
	type args struct {
		ctx               context.Context
		cl                *kadm.Client
		accessControlList *AccessControlList
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Create(tt.args.ctx, tt.args.cl, tt.args.accessControlList); (err != nil) != tt.wantErr {
				t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	type args struct {
		ctx               context.Context
		cl                *kadm.Client
		accessControlList *AccessControlList
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Delete(tt.args.ctx, tt.args.cl, tt.args.accessControlList); (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	type args struct {
		params *v1alpha1.AccessControlListParameters
	}
	var tests []struct {
		name string
		args args
		want *AccessControlList
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Generate(tt.args.params); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Generate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	type args struct {
		in       *v1alpha1.AccessControlListParameters
		observed *AccessControlList
	}
	var tests []struct {
		name string
		args args
		want bool
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsUpToDate(tt.args.in, tt.args.observed); got != tt.want {
				t.Errorf("IsUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestList(t *testing.T) {
	type args struct {
		ctx               context.Context
		cl                *kadm.Client
		accessControlList *AccessControlList
	}
	var tests []struct {
		name    string
		args    args
		want    *AccessControlList
		wantErr bool
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := List(tt.args.ctx, tt.args.cl, tt.args.accessControlList)
			if (err != nil) != tt.wantErr {
				t.Errorf("List() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List() got = %v, want %v", got, tt.want)
			}
		})
	}
}
