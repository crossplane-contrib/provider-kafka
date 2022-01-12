package topic

import (
	"context"
	"testing"

	"github.com/twmb/franz-go/pkg/kadm"

	"github.com/google/go-cmp/cmp"

	"github.com/crossplane/crossplane-runtime/pkg/test"
)

func Test_Get(t *testing.T) {

	type args struct {
		client *kadm.Client
		ctx    context.Context
		name   string
	}

	type want struct {
		*Topic
	}

	cases := map[string]struct {
		reason string
		fields fields
		args   args
		want   want
	}{}

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
