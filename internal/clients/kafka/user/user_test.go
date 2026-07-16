package user

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
)

const (
	testUserAlice   = "alice"
	testUserCharlie = "charlie"
	testPassword    = "secret"
)

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
					capturedUpserts = upsert
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
			username:    testUserAlice,
			mechanisms:  []string{},
			wantDeletes: []kadm.DeleteSCRAM{},
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
					capturedDeletes = del
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
