package topic

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"github.com/crossplane-contrib/provider-kafka/apis/namespaced/topic/v1alpha1"
	common "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/topic"
)

const (
	testTopicID     = "abc-123"
	testRetentionMS = "retention.ms"
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

func TestObserveFirstReconcileNotUpToDate(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }

	cases := map[string]struct {
		reason       string
		existingID   string
		spec         common.TopicParameters
		observed     *topic.Topic
		wantUpToDate bool
	}{
		"EmptyID_SpecMatchesObserved": {
			reason:     "First reconcile (empty ID) must return not-up-to-date to force Update after AddFinalizer",
			existingID: "",
			spec: common.TopicParameters{
				ReplicationFactor: 3,
				Partitions:        6,
				Config:            map[string]*string{testRetentionMS: strPtr("86400000")},
			},
			observed: &topic.Topic{
				ID:                testTopicID,
				ReplicationFactor: 3,
				Partitions:        6,
				Config:            map[string]*string{testRetentionMS: strPtr("86400000")},
			},
			wantUpToDate: false,
		},
		"PopulatedID_SpecMatchesObserved": {
			reason:     "Subsequent reconcile (populated ID) with matching spec should be up-to-date",
			existingID: testTopicID,
			spec: common.TopicParameters{
				ReplicationFactor: 3,
				Partitions:        6,
				Config:            map[string]*string{testRetentionMS: strPtr("86400000")},
			},
			observed: &topic.Topic{
				ID:                testTopicID,
				ReplicationFactor: 3,
				Partitions:        6,
				Config:            map[string]*string{testRetentionMS: strPtr("86400000")},
			},
			wantUpToDate: true,
		},
		"PopulatedID_SpecDiffers": {
			reason:     "Subsequent reconcile with different spec should not be up-to-date",
			existingID: testTopicID,
			spec: common.TopicParameters{
				ReplicationFactor: 3,
				Partitions:        12,
			},
			observed: &topic.Topic{
				ID:                testTopicID,
				ReplicationFactor: 3,
				Partitions:        6,
			},
			wantUpToDate: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cr := &v1alpha1.Topic{}
			cr.Spec.ForProvider = tc.spec
			cr.Status.AtProvider.ID = tc.existingID

			statusPopulated := cr.Status.AtProvider.ID != ""

			got := isResourceUpToDate(cr, statusPopulated, tc.observed)

			assert.Equal(t, tc.wantUpToDate, got, tc.reason)
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
				ID:                testTopicID,
				ReplicationFactor: 3,
				Partitions:        12,
				Config: map[string]*string{
					testRetentionMS:  strPtr("86400000"),
					"cleanup.policy": strPtr("delete"),
				},
			},
			want: common.TopicObservation{
				ID:                testTopicID,
				ReplicationFactor: 3,
				Partitions:        12,
				Config: map[string]*string{
					testRetentionMS:  strPtr("86400000"),
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
			cr.Status.SetConditions(xpv2.Available())

			cr.Status.AtProvider = tc.observed.ToObservation()

			if diff := cmp.Diff(tc.want, cr.Status.AtProvider); diff != "" {
				t.Errorf("\n%s\natProvider: -want, +got:\n%s", tc.reason, diff)
			}
		})
	}
}
