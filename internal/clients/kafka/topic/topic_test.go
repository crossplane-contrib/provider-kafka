package topic

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clustertopic "github.com/crossplane-contrib/provider-kafka/apis/cluster/topic/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
)

const configKeyRetentionMs = "retention.ms"

var dataTesting = []byte(os.Getenv("KAFKA_CONFIG"))

func TestCreate(t *testing.T) {
	newAc, _ := kafka.NewAdminClient(context.Background(), dataTesting, nil)

	type args struct {
		ctx    context.Context
		client *kadm.Client
		topic  *Topic
	}
	{
		cases := map[string]struct {
			name    string
			args    args
			wantErr bool
		}{
			"CreateTopicOne": {
				name: "CreateTopicOne",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              kafka.TestTopicName1,
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},

			"CreateTopicTwo": {
				name: "CreateTopicTwo",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-2",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},

			"CreateTopicThree": {
				name: "CreateTopicThree",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-3",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},
		}

		// TODO: Add test cases.

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				if err := Create(tt.args.ctx, tt.args.client, tt.args.topic); (err != nil) != tt.wantErr {
					t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
}

func TestGet(t *testing.T) {
	newAc, _ := kafka.NewAdminClient(context.Background(), dataTesting, nil)

	type args struct {
		ctx    context.Context
		client *kadm.Client
		name   string
	}
	cases := map[string]struct {
		name    string
		args    args
		want    *Topic
		wantErr bool
	}{
		"GetTopicWorked": {
			name: "GetTopicWorked",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   kafka.TestTopicName1,
			},
			want: &Topic{
				Name:              kafka.TestTopicName1,
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            nil,
			},
			wantErr: false,
		},
		"GetTopicDoesNotExist": {
			name: "GetTopicDoesNotExist",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "testTopic-00",
			},
			want: &Topic{
				Name:              kafka.TestTopicName1,
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Get(tt.args.ctx, tt.args.client, tt.args.name)
			fmt.Println(err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got != nil {
				if got.Name != tt.args.name {
					t.Errorf("Get() Name = %v, want %v", got.Name, tt.args.name)
				}
				if got.Partitions != tt.want.Partitions {
					t.Errorf("Get() Partitions = %v, want %v", got.Partitions, tt.want.Partitions)
				}
				if got.ReplicationFactor != tt.want.ReplicationFactor {
					t.Errorf("Get() ReplicationFactor = %v, want %v", got.ReplicationFactor, tt.want.ReplicationFactor)
				}
				if got.ID == "" {
					t.Errorf("Get() ID should not be empty")
				}
				if got.Config == nil {
					t.Errorf("Get() Config should not be nil")
				}
			}
		})
	}
}

// TestGetAtProviderFields verifies that Get() returns all fields needed for
// status.atProvider population, including topic config entries.
// Uses the Strimzi-managed "pre-existing" topic (cluster/local/kafka-cluster.yaml).
func TestGetAtProviderFields(t *testing.T) {
	if len(dataTesting) == 0 {
		t.Skip("KAFKA_CONFIG not set, skipping integration test")
	}

	ctx := context.Background()
	newAc, err := kafka.NewAdminClient(ctx, dataTesting, nil)
	if err != nil {
		t.Fatalf("failed to create admin client: %v", err)
	}

	got, err := Get(ctx, newAc, "pre-existing")
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}

	if got.Name != "pre-existing" {
		t.Errorf("Name = %q, want %q", got.Name, "pre-existing")
	}
	if got.ID == "" {
		t.Error("ID should not be empty for an existing topic")
	}
	if got.Partitions != 1 {
		t.Errorf("Partitions = %d, want 1", got.Partitions)
	}
	if got.ReplicationFactor != 1 {
		t.Errorf("ReplicationFactor = %d, want 1", got.ReplicationFactor)
	}
	if got.Config == nil {
		t.Fatal("Config should not be nil")
	}
	if _, ok := got.Config[configKeyRetentionMs]; !ok {
		t.Error("Config should contain 'retention.ms' key")
	}
}

func TestGenerate(t *testing.T) {
	type args struct {
		name   string
		params *v1alpha1.TopicParameters
	}

	type want struct {
		topic *Topic
	}

	cases := map[string]struct {
		args args
		want want
	}{
		"ValidComparison": {
			args: args{
				name: "validComparison",
				params: &v1alpha1.TopicParameters{
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
			want: want{
				&Topic{
					Name:              "validComparison",
					ReplicationFactor: 1,
					Partitions:        1,
					Config:            nil,
				},
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			topic := Generate(tt.args.name, tt.args.params)
			fmt.Println(topic)
			if diff := cmp.Diff(tt.want.topic, topic); diff != "" {
				t.Errorf("Generate() =  -want, +got:\n%s", diff)
			}
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	t.Parallel()

	retentionMs := "86400000"
	retentionMsDiff := "172800000"
	segmentBytes := "1073741824"
	cleanupPolicy := "delete"

	cases := map[string]struct {
		in       *v1alpha1.TopicParameters
		observed *Topic
		want     bool
	}{
		"BothNilConfigs": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            nil,
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            nil,
			},
			want: true,
		},
		"DiffReplicationFactor": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 2,
				Partitions:        1,
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
			},
			want: false,
		},
		"DiffPartitions": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        3,
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
			},
			want: false,
		},
		"MatchingConfigs": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMs},
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMs},
			},
			want: true,
		},
		"DiffConfigValue": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMs},
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMsDiff},
			},
			want: false,
		},
		"SpecHasUnknownKey_NotUpToDate": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        1,
				Config: map[string]*string{
					configKeyRetentionMs: &retentionMs,
					"segment.bytes":      &segmentBytes, // broker doesn't return this
				},
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMs},
			},
			want: false,
		},
		"ObservedHasExtraKeys_ServerDefaults": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMs},
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
				Config: map[string]*string{
					configKeyRetentionMs: &retentionMs,
					"cleanup.policy":     &cleanupPolicy,
				},
			},
			want: true,
		},
		"SpecExtraKeys_ButIntersectionDiffers": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        1,
				Config: map[string]*string{
					configKeyRetentionMs: &retentionMs,
					"segment.bytes":      &segmentBytes,
				},
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMsDiff},
			},
			want: false,
		},
		"EmptySpecConfig_ObservedHasConfigs": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{},
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMs},
			},
			want: true,
		},
		"NilSpecConfig_ObservedHasConfigs": {
			in: &v1alpha1.TopicParameters{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            nil,
			},
			observed: &Topic{
				ReplicationFactor: 1,
				Partitions:        1,
				Config:            map[string]*string{configKeyRetentionMs: &retentionMs},
			},
			want: true,
		},
	}
	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := IsUpToDate(tt.in, tt.observed)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("IsUpToDate() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateDuplicateTopic(t *testing.T) {
	newAc, _ := kafka.NewAdminClient(context.Background(), dataTesting, nil)

	fmt.Printf("------Checking duplicate topic creation logic------")

	type args struct {
		ctx    context.Context
		client *kadm.Client
		topic  *Topic
	}
	{
		cases := map[string]struct {
			name    string
			args    args
			wantErr bool
		}{
			"CreateTopicOneExists_Idempotent": {
				name: "CreateTopicOneExists_Idempotent",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              kafka.TestTopicName1,
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},

			"CreateTopicTwoExists_Idempotent": {
				name: "CreateTopicTwoExists_Idempotent",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-2",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},

			"CreateTopicThreeExists_Idempotent": {
				name: "CreateTopicThreeExists_Idempotent",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "testTopic-3",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},
			"CreateTopicDoesNotExist": {
				name: "CreateTopicDoesNotExist",
				args: args{
					ctx:    context.Background(),
					client: newAc,
					topic: &Topic{
						Name:              "createTopic-doesNot-exist",
						ReplicationFactor: 1,
						Partitions:        1,
						Config:            nil,
					},
				},
				wantErr: false,
			},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				if err := Create(tt.args.ctx, tt.args.client, tt.args.topic); (err != nil) != tt.wantErr {
					t.Errorf("Create() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
}

func TestDelete(t *testing.T) {
	newAc, _ := kafka.NewAdminClient(context.Background(), dataTesting, nil)

	type args struct {
		ctx    context.Context
		client *kadm.Client
		name   string
	}
	cases := map[string]struct {
		name    string
		args    args
		wantErr bool
	}{
		"DeleteTopicOne": {
			name: "DeleteTopicOne",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   kafka.TestTopicName1,
			},
			wantErr: false,
		},
		"DeleteTopicTwo": {
			name: "DeleteTopicTwo",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "testTopic-2",
			},
			wantErr: false,
		},
		"DeleteTopicThree": {
			name: "DeleteTopicThree",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "testTopic-3",
			},
			wantErr: false,
		},
		"DeleteTopicThreeDoesNotExist": {
			name: "DeleteTopicThreeDoesNotExist",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "create3",
			},
			wantErr: true,
		},
		"DeleteTopicThatDoesNotExist": {
			name: "DeleteTopicThatDoesNotExist",
			args: args{
				ctx:    context.Background(),
				client: newAc,
				name:   "createTopic-doesNot-exist",
			},
			wantErr: false,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := Delete(tt.args.ctx, tt.args.client, tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPreExistingTopicReconcile verifies that Get() on a Strimzi-created topic
// returns all fields needed to populate status.atProvider when imported via
// crossplane.io/external-name annotation.
func TestPreExistingTopicReconcile(t *testing.T) {
	if len(dataTesting) == 0 {
		t.Skip("KAFKA_CONFIG not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := kafka.NewAdminClient(ctx, dataTesting, nil)
	require.NoError(t, err, "failed to create admin client")

	// "pre-existing" topic is created by Strimzi (cluster/local/kafka-cluster.yaml).
	cr := &clustertopic.Topic{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-cr-name",
		},
	}
	meta.SetExternalName(cr, "pre-existing")

	// Create on an already-existing topic must succeed (idempotent)
	err = Create(ctx, client, &Topic{
		Name:              meta.GetExternalName(cr),
		ReplicationFactor: 1,
		Partitions:        1,
	})
	require.NoError(t, err, "Create on pre-existing topic should be idempotent")

	// Get must return all fields needed for status.atProvider
	got, err := Get(ctx, client, meta.GetExternalName(cr))
	require.NoError(t, err, "Get on pre-existing topic should succeed")

	// Populate status.atProvider the same way the controller does.
	cr.Status.AtProvider = v1alpha1.TopicObservation{
		ID:                got.ID,
		ReplicationFactor: int(got.ReplicationFactor),
		Partitions:        int(got.Partitions),
		Config:            got.Config,
	}

	assert.NotEmpty(t, cr.Status.AtProvider.ID, "status.atProvider.id")
	assert.Equal(t, 1, cr.Status.AtProvider.ReplicationFactor, "status.atProvider.replicationFactor")
	assert.Equal(t, 1, cr.Status.AtProvider.Partitions, "status.atProvider.partitions")
	assert.NotEmpty(t, cr.Status.AtProvider.Config, "status.atProvider.config must contain broker defaults")
}

// TestPreExistingTopicUpdateConfig updates a config key on the Strimzi-managed
// "pre-existing" topic and verifies that a subsequent Get reflects the change
// in all fields needed for status.atProvider.
func TestPreExistingTopicUpdateConfig(t *testing.T) {
	if len(dataTesting) == 0 {
		t.Skip("KAFKA_CONFIG not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := kafka.NewAdminClient(ctx, dataTesting, nil)
	require.NoError(t, err, "failed to create admin client")

	const preExistingTopic = "pre-existing"

	// Read original config to restore after test
	original, err := Get(ctx, client, preExistingTopic)
	require.NoError(t, err)

	newRetention := "172800000"
	err = Update(ctx, client, &Topic{
		Name:              preExistingTopic,
		ReplicationFactor: original.ReplicationFactor,
		Partitions:        original.Partitions,
		Config:            map[string]*string{configKeyRetentionMs: &newRetention},
	})
	require.NoError(t, err, "Update config on pre-existing topic should succeed")

	// Get must reflect the updated config in status.atProvider fields
	got, err := Get(ctx, client, preExistingTopic)
	require.NoError(t, err)

	assert.Equal(t, preExistingTopic, got.Name)
	assert.NotEmpty(t, got.ID, "ID must be populated")
	assert.Equal(t, original.Partitions, got.Partitions)
	assert.Equal(t, original.ReplicationFactor, got.ReplicationFactor)
	require.NotNil(t, got.Config)
	assert.Equal(t, newRetention, *got.Config[configKeyRetentionMs],
		"status.atProvider.config should reflect the updated retention.ms")

	// Restore original value
	t.Cleanup(func() {
		originalRetention := original.Config[configKeyRetentionMs]
		_ = Update(ctx, client, &Topic{
			Name:              preExistingTopic,
			ReplicationFactor: original.ReplicationFactor,
			Partitions:        original.Partitions,
			Config:            map[string]*string{configKeyRetentionMs: originalRetention},
		})
	})
}

func TestUpdatePartitions_DecreasePartitionsFails(t *testing.T) {
	t.Parallel()

	err := updatePartitions(context.Background(), nil, &Topic{Partitions: 3}, &Topic{Partitions: 5})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCannotDecreasePartitions)
	assert.Contains(t, err.Error(), "from 5 to 3")
}

func TestToObservation(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }

	tpc := &Topic{
		ID:                "abc-123",
		ReplicationFactor: 3,
		Partitions:        12,
		Config: map[string]*string{
			configKeyRetentionMs: strPtr("86400000"),
			"cleanup.policy":     strPtr("delete"),
		},
	}

	got := tpc.ToObservation()

	assert.Equal(t, tpc.ID, got.ID)
	assert.Equal(t, int(tpc.ReplicationFactor), got.ReplicationFactor)
	assert.Equal(t, int(tpc.Partitions), got.Partitions)
	assert.Equal(t, tpc.Config, got.Config)
}
