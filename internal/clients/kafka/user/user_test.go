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
			input: "SCRAM-SHA-512",
			want:  kadm.ScramSha512,
		},
		"SHA256": {
			input: "SCRAM-SHA-256",
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
			observed: []string{"SCRAM-SHA-512"},
			desired:  []string{"SCRAM-SHA-512"},
			want:     true,
		},
		"DifferentMechanism": {
			observed: []string{"SCRAM-SHA-512"},
			desired:  []string{"SCRAM-SHA-256"},
			want:     false,
		},
		"OrderInsensitive": {
			observed: []string{"SCRAM-SHA-256", "SCRAM-SHA-512"},
			desired:  []string{"SCRAM-SHA-512", "SCRAM-SHA-256"},
			want:     true,
		},
		"MissingMechanism": {
			observed: []string{"SCRAM-SHA-512"},
			desired:  []string{"SCRAM-SHA-512", "SCRAM-SHA-256"},
			want:     false,
		},
		"ExtraMechanism": {
			observed: []string{"SCRAM-SHA-512", "SCRAM-SHA-256"},
			desired:  []string{"SCRAM-SHA-512"},
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

func TestExists(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		username   string
		describeFn func(ctx context.Context, users ...string) (kadm.DescribedUserSCRAMs, error)
		want       bool
		wantErr    bool
	}{
		"UserExists": {
			username: "alice",
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{
					"alice": {
						User: "alice",
						CredInfos: []kadm.CredInfo{
							{Mechanism: kadm.ScramSha512},
						},
					},
				}, nil
			},
			want: true,
		},
		"UserAbsent": {
			username: "bob",
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{}, nil
			},
			want: false,
		},
		"UserResourceNotFound": {
			username: "charlie",
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{
					"charlie": {
						User: "charlie",
						Err:  kerr.ResourceNotFound,
					},
				}, nil
			},
			want: false,
		},
		"UserWithOtherKafkaError": {
			username: "charlie",
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{
					"charlie": {
						User: "charlie",
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
			got, err := Exists(context.Background(), cl, tc.username)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestObservedMechanisms(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		username   string
		describeFn func(ctx context.Context, users ...string) (kadm.DescribedUserSCRAMs, error)
		want       []string
		wantErr    bool
	}{
		"TwoMechanisms": {
			username: "alice",
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{
					"alice": {
						User: "alice",
						CredInfos: []kadm.CredInfo{
							{Mechanism: kadm.ScramSha512},
							{Mechanism: kadm.ScramSha256},
						},
					},
				}, nil
			},
			want: []string{"SCRAM-SHA-512", "SCRAM-SHA-256"},
		},
		"UserAbsent": {
			username: "bob",
			describeFn: func(_ context.Context, users ...string) (kadm.DescribedUserSCRAMs, error) {
				return kadm.DescribedUserSCRAMs{}, nil
			},
			want: nil,
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
			got, err := ObservedMechanisms(context.Background(), cl, tc.username)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
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
			username:   "alice",
			password:   "secret",
			mechanisms: []string{"SCRAM-SHA-512"},
			wantUpserts: []kadm.UpsertSCRAM{
				{User: "alice", Mechanism: kadm.ScramSha512, Iterations: defaultScramIterations, Password: "secret"},
			},
		},
		"BothMechanisms": {
			username:   "alice",
			password:   "secret",
			mechanisms: []string{"SCRAM-SHA-256", "SCRAM-SHA-512"},
			wantUpserts: []kadm.UpsertSCRAM{
				{User: "alice", Mechanism: kadm.ScramSha256, Iterations: defaultScramIterations, Password: "secret"},
				{User: "alice", Mechanism: kadm.ScramSha512, Iterations: defaultScramIterations, Password: "secret"},
			},
		},
		"UnknownMechanism": {
			username:   "alice",
			password:   "secret",
			mechanisms: []string{"SCRAM-SHA-999"},
			wantErr:    true,
		},
		"AlterError": {
			username:   "alice",
			password:   "secret",
			mechanisms: []string{"SCRAM-SHA-512"},
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
			username:   "alice",
			mechanisms: []string{"SCRAM-SHA-512"},
			wantDeletes: []kadm.DeleteSCRAM{
				{User: "alice", Mechanism: kadm.ScramSha512},
			},
		},
		"BothMechanisms": {
			username:   "alice",
			mechanisms: []string{"SCRAM-SHA-256", "SCRAM-SHA-512"},
			wantDeletes: []kadm.DeleteSCRAM{
				{User: "alice", Mechanism: kadm.ScramSha256},
				{User: "alice", Mechanism: kadm.ScramSha512},
			},
		},
		"EmptyMechanisms": {
			username:    "alice",
			mechanisms:  []string{},
			wantDeletes: []kadm.DeleteSCRAM{},
		},
		"UnknownMechanism": {
			username:   "alice",
			mechanisms: []string{"SCRAM-SHA-999"},
			wantErr:    true,
		},
		"AlterError": {
			username:   "alice",
			mechanisms: []string{"SCRAM-SHA-512"},
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
