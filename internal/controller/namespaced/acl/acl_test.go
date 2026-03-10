/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package acl

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane-contrib/provider-kafka/apis/namespaced/acl/v1alpha1"
	common "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	aclclient "github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/acl"
)

func TestObserveWrongType(t *testing.T) {
	type want struct {
		o   managed.ExternalObservation
		err error
	}

	cases := map[string]struct {
		reason string
		want   want
	}{
		"NotAnACL": {
			reason: "Should return error when managed resource is not an AccessControlList",
			want: want{
				o:   managed.ExternalObservation{},
				err: errors.New(errNotAccessControlList),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{}
			got, err := e.Observe(context.Background(), &fake.Managed{})
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want error, +got error:\n%s", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}

func TestObserveNoExternalName(t *testing.T) {
	cr := &v1alpha1.AccessControlList{}

	e := external{}
	got, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	want := managed.ExternalObservation{ResourceExists: false}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Observe() with no external name: -want, +got:\n%s", diff)
	}
}

func TestPopulateACLAtProvider(t *testing.T) {
	cases := map[string]struct {
		reason   string
		observed *aclclient.AccessControlList
		want     common.AccessControlListObservation
	}{
		"AllFieldsPopulated": {
			reason: "All observed ACL fields should be mapped to atProvider",
			observed: &aclclient.AccessControlList{
				ResourceName:              "my-topic",
				ResourceType:              "Topic",
				ResourcePrincipal:         "User:alice",
				ResourceHost:              "*",
				ResourceOperation:         "Read",
				ResourcePermissionType:    "Allow",
				ResourcePatternTypeFilter: "Literal",
			},
			want: common.AccessControlListObservation{
				ResourceName:              "my-topic",
				ResourceType:              "Topic",
				ResourcePrincipal:         "User:alice",
				ResourceHost:              "*",
				ResourceOperation:         "Read",
				ResourcePermissionType:    "Allow",
				ResourcePatternTypeFilter: "Literal",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr := &v1alpha1.AccessControlList{}
			cr.Status.SetConditions(v1.Available())

			cr.Status.AtProvider.ResourceName = tc.observed.ResourceName
			cr.Status.AtProvider.ResourceType = tc.observed.ResourceType
			cr.Status.AtProvider.ResourcePrincipal = tc.observed.ResourcePrincipal
			cr.Status.AtProvider.ResourceHost = tc.observed.ResourceHost
			cr.Status.AtProvider.ResourceOperation = tc.observed.ResourceOperation
			cr.Status.AtProvider.ResourcePermissionType = tc.observed.ResourcePermissionType
			cr.Status.AtProvider.ResourcePatternTypeFilter = tc.observed.ResourcePatternTypeFilter

			if diff := cmp.Diff(tc.want, cr.Status.AtProvider); diff != "" {
				t.Errorf("\n%s\natProvider: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
