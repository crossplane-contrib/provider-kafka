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

package topic

import (
	"context"
	"github.com/crossplane/crossplane-runtime/pkg/resource/fake"
	"github.com/pkg/errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kadm"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/test"
)

// Unlike many Kubernetes projects Crossplane does not use third party testing
// libraries, per the common Go test review comments. Crossplane encourages the
// use of table driven unit tests. The tests of the crossplane-runtime project
// are representative of the testing style Crossplane encourages.
//
// https://github.com/golang/go/wiki/TestComments
// https://github.com/crossplane/crossplane/blob/master/CONTRIBUTING.md#contributing-code

func TestObserve(t *testing.T) {
	type fields struct {
		service *kadm.Client
	}

	type args struct {
		ctx context.Context
		mg  resource.Managed
		input string
	}

	type want struct {
		o   managed.ExternalObservation
		err error
		output string
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{
		// testing that topic does not exist
		"TopicDoesNotExist": {
			reason: "Testing that a resource does not exist.",
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{},
			},
			want: want{
				o: managed.ExternalObservation{
					ResourceExists: false,
					ResourceUpToDate: false,
					ResourceLateInitialized: false,
				},
				err: errors.New ("managed resource is not a Topic custom resource"),
			},
		},

		"TopicExistsNotUpToDate": {
			reason: "Testing that topic exists but is not up to date.",
			args: args{
				ctx: context.Background(),
				mg: &fake.Managed{},
			},
			want: want{
				o: managed.ExternalObservation{
					ResourceExists: true,
					ResourceUpToDate: false,
					ResourceLateInitialized: false,
				},
			},
		},



	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := external{kafkaClient: tc.fields.service}
			got, err := e.Observe(tc.args.ctx, tc.args.mg)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want error, +got error:\n%s\n", tc.reason, diff)
			}
			if diff := cmp.Diff(tc.want.o, got); diff != "" {
				t.Errorf("\n%s\ne.Observe(...): -want, +got:\n%s\n", tc.reason, diff)
			}
		})
	}
}
