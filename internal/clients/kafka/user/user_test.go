package user

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"

	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
)

const (
	testUserAlice   = "alice"
	testUserCharlie = "charlie"
	testPassword    = "secret"
)

// dataTesting holds the provider config JSON for the live Kafka cluster that
// "make test" spins up. Empty outside that target, which skips the tests below
// that need a broker.
var dataTesting = []byte(os.Getenv("KAFKA_CONFIG"))

// integrationClient skips the test unless KAFKA_CONFIG points at a live Kafka,
// then connects to it.
func integrationClient(t *testing.T) (context.Context, ScramClient) {
	t.Helper()
	if len(dataTesting) == 0 {
		t.Skip("KAFKA_CONFIG not set, skipping integration test")
	}
	ctx := context.Background()
	cl, err := kafka.NewAdminClient(ctx, dataTesting, nil)
	require.NoError(t, err, "failed to create admin client")
	return ctx, cl
}

// fakeScramClient implements ScramClient for unit testing.
type fakeScramClient struct {
	describeFn func(ctx context.Context, users ...string) (kadm.DescribedUserSCRAMs, error)
	alterFn    func(ctx context.Context, del []kadm.DeleteSCRAM, upsert []kadm.UpsertSCRAM) (kadm.AlteredUserSCRAMs, error)
}

func (f *fakeScramClient) DescribeUserSCRAMs(ctx context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
	return f.describeFn(ctx, users...)
}

func (f *fakeScramClient) AlterUserSCRAMs(ctx context.Context, del []kadm.DeleteSCRAM, upsert []kadm.UpsertSCRAM) (kadm.AlteredUserSCRAMs, error) {
	return f.alterFn(ctx, del, upsert)
}

func TestMechanismFromString(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		input   string
		want    kadm.ScramMechanism
		wantErr bool
	}{
		"SHA512": {
			input: mechanismSHA512,
			want:  kadm.ScramSha512,
		},
		"SHA256": {
			input: mechanismSHA256,
			want:  kadm.ScramSha256,
		},
		"Unknown": {
			input:   "SCRAM-SHA-999",
			wantErr: true,
		},
		"Empty": {
			input:   "",
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := MechanismFromString(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsUpToDate(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		observed []string
		desired  []string
		want     bool
	}{
		"MatchSingleMechanism": {
			observed: []string{mechanismSHA512},
			desired:  []string{mechanismSHA512},
			want:     true,
		},
		"DifferentMechanism": {
			observed: []string{mechanismSHA512},
			desired:  []string{mechanismSHA256},
			want:     false,
		},
		"OrderInsensitive": {
			observed: []string{mechanismSHA256, mechanismSHA512},
			desired:  []string{mechanismSHA512, mechanismSHA256},
			want:     true,
		},
		"MissingMechanism": {
			observed: []string{mechanismSHA512},
			desired:  []string{mechanismSHA512, mechanismSHA256},
			want:     false,
		},
		"ExtraMechanism": {
			observed: []string{mechanismSHA512, mechanismSHA256},
			desired:  []string{mechanismSHA512},
			want:     false,
		},
		"BothEmpty": {
			observed: []string{},
			desired:  []string{},
			want:     true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := IsUpToDate(tc.observed, tc.desired)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDescribe(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		username   string
		describeFn func(ctx context.Context, users ...string) (kadm.DescribedUserSCRAMs, error)
		wantExists bool
		wantMechs  []string
		wantErr    bool
	}{
		"UserExists": {
			username: testUserAlice,
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{
					testUserAlice: {
						User: testUserAlice,
						CredInfos: []kadm.CredInfo{
							{Mechanism: kadm.ScramSha512},
						},
					},
				}, nil
			},
			wantExists: true,
			wantMechs:  []string{mechanismSHA512},
		},
		"TwoMechanisms": {
			username: testUserAlice,
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{
					testUserAlice: {
						User: testUserAlice,
						CredInfos: []kadm.CredInfo{
							{Mechanism: kadm.ScramSha512},
							{Mechanism: kadm.ScramSha256},
						},
					},
				}, nil
			},
			wantExists: true,
			wantMechs:  []string{mechanismSHA512, mechanismSHA256},
		},
		"UserAbsent": {
			username: "bob",
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{}, nil
			},
			wantExists: false,
			wantMechs:  nil,
		},
		"UserResourceNotFound": {
			username: testUserCharlie,
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{
					testUserCharlie: {
						User: testUserCharlie,
						Err:  kerr.ResourceNotFound,
					},
				}, nil
			},
			wantExists: false,
			wantMechs:  nil,
		},
		"UserWithOtherKafkaError": {
			username: testUserCharlie,
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{
					testUserCharlie: {
						User: testUserCharlie,
						Err:  errors.New("some other kafka error"),
					},
				}, nil
			},
			wantErr: true,
		},
		"DescribeError": {
			username: "dave",
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return nil, errors.New("kafka unavailable")
			},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cl := &fakeScramClient{describeFn: tc.describeFn}
			gotExists, gotMechs, err := Describe(context.Background(), cl, tc.username)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantExists, gotExists)
			assert.Equal(t, tc.wantMechs, gotMechs)
		})
	}
}

func TestExists(t *testing.T) {
	t.Parallel()

	cl := &fakeScramClient{
		describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
			return kadm.DescribedUserSCRAMs{
				testUserAlice: {
					User: testUserAlice,
					CredInfos: []kadm.CredInfo{
						{Mechanism: kadm.ScramSha512},
					},
				},
			}, nil
		},
	}
	got, err := Exists(context.Background(), cl, testUserAlice)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestObservedMechanisms(t *testing.T) {
	t.Parallel()

	cl := &fakeScramClient{
		describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
			return kadm.DescribedUserSCRAMs{
				testUserAlice: {
					User: testUserAlice,
					CredInfos: []kadm.CredInfo{
						{Mechanism: kadm.ScramSha512},
					},
				},
			}, nil
		},
	}
	got, err := ObservedMechanisms(context.Background(), cl, testUserAlice)
	require.NoError(t, err)
	assert.Equal(t, []string{mechanismSHA512}, got)
}

func TestUpsert(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		username    string
		password    string
		mechanisms  []string
		wantUpserts []kadm.UpsertSCRAM
		alterErr    error
		wantErr     bool
	}{
		"SingleMechanism": {
			username:   testUserAlice,
			password:   testPassword,
			mechanisms: []string{mechanismSHA512},
			wantUpserts: []kadm.UpsertSCRAM{
				{User: testUserAlice, Mechanism: kadm.ScramSha512, Iterations: defaultScramIterations, Password: testPassword},
			},
		},
		"BothMechanisms": {
			username:   testUserAlice,
			password:   testPassword,
			mechanisms: []string{mechanismSHA256, mechanismSHA512},
			wantUpserts: []kadm.UpsertSCRAM{
				{User: testUserAlice, Mechanism: kadm.ScramSha256, Iterations: defaultScramIterations, Password: testPassword},
				{User: testUserAlice, Mechanism: kadm.ScramSha512, Iterations: defaultScramIterations, Password: testPassword},
			},
		},
		"UnknownMechanism": {
			username:   testUserAlice,
			password:   testPassword,
			mechanisms: []string{"SCRAM-SHA-999"},
			wantErr:    true,
		},
		"AlterError": {
			username:   testUserAlice,
			password:   testPassword,
			mechanisms: []string{mechanismSHA512},
			alterErr:   errors.New("alter failed"),
			wantErr:    true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var capturedUpserts []kadm.UpsertSCRAM
			cl := &fakeScramClient{
				alterFn: func(_ context.Context, del []kadm.DeleteSCRAM, upsert []kadm.UpsertSCRAM) (kadm.AlteredUserSCRAMs, error) {
					// Kafka forbids naming a user twice in one request.
					require.Len(t, upsert, 1)
					capturedUpserts = append(capturedUpserts, upsert...)
					if tc.alterErr != nil {
						return nil, tc.alterErr
					}
					return kadm.AlteredUserSCRAMs{}, nil
				},
			}

			err := Upsert(context.Background(), cl, tc.username, tc.password, tc.mechanisms)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantUpserts, capturedUpserts)
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		username    string
		mechanisms  []string
		wantDeletes []kadm.DeleteSCRAM
		alterErr    error
		wantErr     bool
	}{
		"SingleMechanism": {
			username:   testUserAlice,
			mechanisms: []string{mechanismSHA512},
			wantDeletes: []kadm.DeleteSCRAM{
				{User: testUserAlice, Mechanism: kadm.ScramSha512},
			},
		},
		"BothMechanisms": {
			username:   testUserAlice,
			mechanisms: []string{mechanismSHA256, mechanismSHA512},
			wantDeletes: []kadm.DeleteSCRAM{
				{User: testUserAlice, Mechanism: kadm.ScramSha256},
				{User: testUserAlice, Mechanism: kadm.ScramSha512},
			},
		},
		"EmptyMechanisms": {
			username:   testUserAlice,
			mechanisms: []string{},
		},
		"UnknownMechanism": {
			username:   testUserAlice,
			mechanisms: []string{"SCRAM-SHA-999"},
			wantErr:    true,
		},
		"AlterError": {
			username:   testUserAlice,
			mechanisms: []string{mechanismSHA512},
			alterErr:   errors.New("alter failed"),
			wantErr:    true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var capturedDeletes []kadm.DeleteSCRAM
			cl := &fakeScramClient{
				alterFn: func(_ context.Context, del []kadm.DeleteSCRAM, upsert []kadm.UpsertSCRAM) (kadm.AlteredUserSCRAMs, error) {
					// Kafka forbids naming a user twice in one request.
					require.Len(t, del, 1)
					capturedDeletes = append(capturedDeletes, del...)
					if tc.alterErr != nil {
						return nil, tc.alterErr
					}
					return kadm.AlteredUserSCRAMs{}, nil
				},
			}

			err := Delete(context.Background(), cl, tc.username, tc.mechanisms)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantDeletes, capturedDeletes)
		})
	}
}

// TestPreExistingUserDescribe verifies that a user created outside the provider
// (the Strimzi KafkaUser "user" from cluster/local/kafka-cluster.yaml) can be
// imported: Describe must report it as existing with its enrolled mechanisms.
func TestPreExistingUserDescribe(t *testing.T) {
	ctx, cl := integrationClient(t)

	exists, mechs, err := Describe(ctx, cl, "user")
	require.NoError(t, err)
	require.True(t, exists, "Strimzi-managed user should exist")
	assert.Equal(t, []string{mechanismSHA512}, mechs)
	assert.True(t, IsUpToDate(mechs, []string{mechanismSHA512}))
}

// TestUserLifecycle covers create, update (mechanism added) and delete against
// a live Kafka. Uses a provider-owned user so the Strimzi user operator does
// not fight over it.
func TestUserLifecycle(t *testing.T) {
	ctx, cl := integrationClient(t)

	const username = "provider-test-user"
	t.Cleanup(func() { _ = Delete(ctx, cl, username, []string{mechanismSHA512, mechanismSHA256}) })

	// Not there yet.
	exists, _, err := Describe(ctx, cl, username)
	require.NoError(t, err)
	require.False(t, exists, "test user must not exist before the test")

	// Create.
	require.NoError(t, Upsert(ctx, cl, username, testPassword, []string{mechanismSHA512}))
	requireMechanisms(ctx, t, cl, username, mechanismSHA512)

	// Update: enroll a second mechanism.
	require.NoError(t, Upsert(ctx, cl, username, testPassword, []string{mechanismSHA512, mechanismSHA256}))
	requireMechanisms(ctx, t, cl, username, mechanismSHA256, mechanismSHA512)

	// Delete.
	require.NoError(t, Delete(ctx, cl, username, []string{mechanismSHA512, mechanismSHA256}))
	require.Eventually(t, func() bool {
		exists, _, err := Describe(ctx, cl, username)
		return err == nil && !exists
	}, 10*time.Second, 200*time.Millisecond, "user still exists after Delete")
}

// requireMechanisms waits until Describe reports exactly the given mechanisms;
// SCRAM changes propagate through the cluster metadata asynchronously.
func requireMechanisms(ctx context.Context, t *testing.T, cl ScramClient, username string, want ...string) {
	t.Helper()
	var got []string
	require.Eventuallyf(t, func() bool {
		exists, mechs, err := Describe(ctx, cl, username)
		got = mechs
		return err == nil && exists && IsUpToDate(mechs, want)
	}, 10*time.Second, 200*time.Millisecond, "want mechanisms %v, last observed %v", want, &got)
}
