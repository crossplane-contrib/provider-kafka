package acl

import (
	"context"
	"github.com/crossplane-contrib/provider-kafka/apis/acl/v1alpha1"
	"github.com/twmb/franz-go/pkg/kadm"
	"k8s.io/apimachinery/pkg/util/json"
	"reflect"
	"testing"
)

var baseAcl = AccessControlList{
	Name:                      "acl1",
	ResourceType:              "Topic",
	Principle:                 "User:Ken",
	Host:                      "*",
	Operation:                 "AlterConfigs",
	PermissionType:            "Allow",
	ResourcePatternTypeFilter: "Literal",
}

var baseJsonAcl = `{	"Name": "acl",
		"ResourceType": "Topic",
		"Principle": "User:Ken",
		"Host": "*", "Operation":
		"AlterConfigs",
		"PermissionType": "Allow",
		"ResourcePatternTypeFilter": "Literal"

		}`

func TestCompareAcls(t *testing.T) {
	type args struct {
		extname  AccessControlList
		observed AccessControlList
	}

	aclTesting := baseAcl

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
					Name:                      "acl1",
					ResourceType:              "Topic",
					Principle:                 "User:Ken",
					Host:                      "*",
					Operation:                 "AlterConfigs",
					PermissionType:            "Allow",
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
					Name:                      "acl10",
					ResourceType:              "Topic",
					Principle:                 "User:Ken",
					Host:                      "*",
					Operation:                 "AlterConfigs",
					PermissionType:            "Allow",
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
					Name:                      "acl1",
					ResourceType:              "Topical",
					Principle:                 "User:Ken",
					Host:                      "*",
					Operation:                 "AlterConfigs",
					PermissionType:            "Allow",
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
					Name:                      "acl1",
					ResourceType:              "Topic",
					Principle:                 "User:NotKen",
					Host:                      "*",
					Operation:                 "AlterConfigs",
					PermissionType:            "Allow",
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
					Name:                      "acl1",
					ResourceType:              "Topic",
					Principle:                 "User:Ken",
					Host:                      "acme.com",
					Operation:                 "AlterConfigs",
					PermissionType:            "Allow",
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
					Name:                      "acl1",
					ResourceType:              "Topic",
					Principle:                 "User:Ken",
					Host:                      "*",
					Operation:                 "Read",
					PermissionType:            "Allow",
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
					Name:                      "acl1",
					ResourceType:              "Topic",
					Principle:                 "User:Ken",
					Host:                      "*",
					Operation:                 "AlterConfigs",
					PermissionType:            "Write",
					ResourcePatternTypeFilter: "Any",
				},
			},
			want: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompareAcls(tt.args.extname, tt.args.observed); got != tt.want {
				t.Errorf("CompareAcls() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertFromJSON(t *testing.T) {
	type args struct {
		extname string
	}

	baseAcl := baseAcl
	//baseJsonAcl := baseJsonAcl

	//

	cases := []struct {
		name    string
		args    args
		want    *AccessControlList
		wantErr bool
	}{
		{
			name: "Hello",
			args: args{
				extname: "acl1",
			},
			want:    &baseAcl,
			wantErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertFromJSON(tt.args.extname)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertFromJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertFromJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToJSON(t *testing.T) {
	type args struct {
		acl *AccessControlList
	}

	//aclJsonConvert := baseAcl
	aclJsonMarshal, _ := json.Marshal(baseAcl)
	aclJsonString := string(aclJsonMarshal)

	cases := map[string]struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		"PassJsonMarshal": {
			name: "SuccessfulMarshal",
			args: args{
				acl: &baseAcl,
			},
			want:    aclJsonString,
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
		name   string
		params *v1alpha1.AccessControlListParameters
	}
	tests := []struct {
		name string
		args args
		want *AccessControlList
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Generate(tt.args.name, tt.args.params); !reflect.DeepEqual(got, tt.want) {
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
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
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
	tests := []struct {
		name    string
		args    args
		want    *AccessControlList
		wantErr bool
	}{
		// TODO: Add test cases.
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
