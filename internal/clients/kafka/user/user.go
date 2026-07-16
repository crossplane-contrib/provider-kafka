package user

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
)

const (
	errUnknownMechanism = "unknown SCRAM mechanism"
	errDescribeUser     = "cannot describe SCRAM credentials"
	errAlterUser        = "cannot alter SCRAM credentials"

	// defaultScramIterations is the default SCRAM iteration count sent to Kafka.
	// Kafka requires a value between 4096 and 16384; 4096 is the minimum and
	// matches the server-side default for most distributions.
	defaultScramIterations = 4096

	mechanismSHA256 = "SCRAM-SHA-256"
	mechanismSHA512 = "SCRAM-SHA-512"
)

// ScramClient abstracts the kadm SCRAM operations needed by this package.
// *kadm.Client satisfies this interface.
type ScramClient interface {
	DescribeUserSCRAMs(ctx context.Context, users ...string) (kadm.DescribedUserSCRAMs, error)
	AlterUserSCRAMs(ctx context.Context, del []kadm.DeleteSCRAM, upsert []kadm.UpsertSCRAM) (kadm.AlteredUserSCRAMs, error)
}

// MechanismFromString converts a SCRAM mechanism name to its kadm constant.
// Accepted values: "SCRAM-SHA-256", "SCRAM-SHA-512".
func MechanismFromString(s string) (kadm.ScramMechanism, error) {
	switch strings.ToUpper(s) {
	case mechanismSHA256:
		return kadm.ScramSha256, nil
	case mechanismSHA512:
		return kadm.ScramSha512, nil
	default:
		return 0, fmt.Errorf("%s: %q", errUnknownMechanism, s)
	}
}

// IsUpToDate returns true if the observed and desired mechanism sets are equal
// (order-insensitive).
func IsUpToDate(observed, desired []string) bool {
	if len(observed) != len(desired) {
		return false
	}
	o := make([]string, len(observed))
	d := make([]string, len(desired))
	copy(o, observed)
	copy(d, desired)
	sort.Strings(o)
	sort.Strings(d)
	for i := range o {
		if o[i] != d[i] {
			return false
		}
	}
	return true
}

// Describe returns whether the user exists and its enrolled mechanisms in a
// single Kafka RPC. Returns (false, nil, nil) when the user has no credentials.
func Describe(ctx context.Context, cl ScramClient, username string) (exists bool, mechanisms []string, err error) {
	resp, err := cl.DescribeUserSCRAMs(ctx, username)
	if err != nil {
		return false, nil, fmt.Errorf("%s: %w", errDescribeUser, err)
	}
	described, ok := resp[username]
	if !ok {
		return false, nil, nil
	}
	if described.Err != nil {
		if errors.Is(described.Err, kerr.ResourceNotFound) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("%s: %w", errDescribeUser, described.Err)
	}
	if len(described.CredInfos) == 0 {
		return false, nil, nil
	}
	mechs := make([]string, 0, len(described.CredInfos))
	for _, ci := range described.CredInfos {
		mechs = append(mechs, ci.Mechanism.String())
	}
	return true, mechs, nil
}

// Exists returns true when the user has at least one SCRAM credential enrolled
// in Kafka.
func Exists(ctx context.Context, cl ScramClient, username string) (bool, error) {
	exists, _, err := Describe(ctx, cl, username)
	return exists, err
}

// ObservedMechanisms returns the mechanism names currently enrolled for the
// user, as reported by Kafka. Returns nil if the user has no credentials.
func ObservedMechanisms(ctx context.Context, cl ScramClient, username string) ([]string, error) {
	_, mechs, err := Describe(ctx, cl, username)
	return mechs, err
}

// Upsert creates or updates SCRAM credentials for the user with the given
// mechanisms and password. SCRAM iteration count is set to defaultScramIterations (4096).
func Upsert(ctx context.Context, cl ScramClient, username, password string, mechanisms []string) error {
	upserts := make([]kadm.UpsertSCRAM, 0, len(mechanisms))
	for _, m := range mechanisms {
		mech, err := MechanismFromString(m)
		if err != nil {
			return err
		}
		upserts = append(upserts, kadm.UpsertSCRAM{
			User:       username,
			Mechanism:  mech,
			Iterations: defaultScramIterations,
			Password:   password,
		})
	}
	results, err := cl.AlterUserSCRAMs(ctx, nil, upserts)
	if err != nil {
		return fmt.Errorf("%s: %w", errAlterUser, err)
	}
	if err := results.Error(); err != nil {
		return fmt.Errorf("%s: %w", errAlterUser, err)
	}
	return nil
}

// Delete removes SCRAM credentials for each mechanism in the list. Mechanisms
// should be taken from status.atProvider so that all enrolled credentials are
// cleaned up regardless of the current spec.
func Delete(ctx context.Context, cl ScramClient, username string, mechanisms []string) error {
	deletions := make([]kadm.DeleteSCRAM, 0, len(mechanisms))
	for _, m := range mechanisms {
		mech, err := MechanismFromString(m)
		if err != nil {
			return err
		}
		deletions = append(deletions, kadm.DeleteSCRAM{
			User:      username,
			Mechanism: mech,
		})
	}
	results, err := cl.AlterUserSCRAMs(ctx, deletions, nil)
	if err != nil {
		return fmt.Errorf("%s: %w", errAlterUser, err)
	}
	if err := results.Error(); err != nil {
		return fmt.Errorf("%s: %w", errAlterUser, err)
	}
	return nil
}
