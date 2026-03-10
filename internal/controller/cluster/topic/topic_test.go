package topic

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"

	"github.com/crossplane-contrib/provider-kafka/apis/cluster/topic/v1alpha1"
	common "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/topic"
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
		"NotATopic": {
			reason: "Should return error when managed resource is not a Topic",
			want: want{
				o:   managed.ExternalObservation{},
				err: errors.New(errNotTopic),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := &external{}
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

func TestPopulateTopicAtProvider(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	cases := map[string]struct {
		reason   string
		observed *topic.Topic
		want     common.TopicObservation
	}{
		"AllFieldsPopulated": {
			reason: "All observed fields should be mapped to atProvider",
			observed: &topic.Topic{
				ID:                "abc-123",
				ReplicationFactor: 3,
				Partitions:        12,
				Config: map[string]*string{
					"retention.ms":   strPtr("86400000"),
					"cleanup.policy": strPtr("delete"),
				},
			},
			want: common.TopicObservation{
				ID:                "abc-123",
				ReplicationFactor: 3,
				Partitions:        12,
				Config: map[string]*string{
					"retention.ms":   strPtr("86400000"),
					"cleanup.policy": strPtr("delete"),
				},
			},
		},
		"MinimalFields": {
			reason: "Observation should work with minimal data and nil config",
			observed: &topic.Topic{
				ID:                "def-456",
				ReplicationFactor: 1,
				Partitions:        1,
			},
			want: common.TopicObservation{
				ID:                "def-456",
				ReplicationFactor: 1,
				Partitions:        1,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr := &v1alpha1.Topic{}
			cr.Status.SetConditions(v1.Available())

			cr.Status.AtProvider.ID = tc.observed.ID
			cr.Status.AtProvider.ReplicationFactor = int(tc.observed.ReplicationFactor)
			cr.Status.AtProvider.Partitions = int(tc.observed.Partitions)
			cr.Status.AtProvider.Config = tc.observed.Config

			if diff := cmp.Diff(tc.want, cr.Status.AtProvider); diff != "" {
				t.Errorf("\n%s\natProvider: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
