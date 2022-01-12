package topic

import (
	"context"
	"testing"

	"github.com/twmb/franz-go/pkg/kadm"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
)

type kdmClient struct {
	client *kadm.Client
}

func Test_Get(t *testing.T) {

	type args struct {
		ctx  context.Context
		name string
	}

	cases := map[string]struct {
		args              args
		Name              string
		ReplicationFactor int16
		Partitions        int32
		ID                string
		Config            map[string]*string
	}{}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := tc.kdmClient.client
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

func Test_Create(t *testing.T) {

	type args struct {
		ctx  context.Context
		name string
	}

	cases := map[string]struct {
		args              args
		Name              string
		ReplicationFactor int16
		Partitions        int32
		ID                string
		Config            map[string]*string
	}{}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := tc.kdmClient.client
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
