/*
Copyright 2026 The Crossplane Authors.

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

package user

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	apis "github.com/crossplane-contrib/provider-kafka/apis/cluster"
	"github.com/crossplane-contrib/provider-kafka/apis/cluster/user/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/cluster/v1alpha1"
	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	userhelpers "github.com/crossplane-contrib/provider-kafka/internal/controller/user"
)

const (
	mechanismSHA256 = "SCRAM-SHA-256"
	mechanismSHA512 = "SCRAM-SHA-512"
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
		"NotAUser": {
			reason: "Should return error when managed resource is not a User",
			want: want{
				o:   managed.ExternalObservation{},
				err: errors.New(userhelpers.ErrNotUser),
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

func TestResolvePassword(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cases := map[string]struct {
		cr      *v1alpha1.User
		secrets []runtime.Object
		wantPw  string // empty means: assert a 32-char generated password
		wantErr bool
	}{
		"PasswordSecretRef": {
			cr: userWithPasswordRef("my-secret", "default", "password"),
			secrets: []runtime.Object{
				secret("my-secret", "default", map[string][]byte{"password": []byte("s3cr3t!")}),
			},
			wantPw: "s3cr3t!",
		},
		"PasswordSecretRefMissing": {
			cr:      userWithPasswordRef("missing-secret", "default", "password"),
			secrets: []runtime.Object{},
			wantErr: true,
		},
		"ReuseFromOutputSecret": {
			cr: userWithWriteRef("out-secret", "default"),
			secrets: []runtime.Object{
				secret("out-secret", "default", map[string][]byte{"password": []byte("kept-password")}),
			},
			wantPw: "kept-password",
		},
		"AutoGenerateWhenNoOutputSecret": {
			cr:      userWithWriteRef("non-existent", "default"),
			secrets: []runtime.Object{},
			// wantPw empty → assert 32-char generated
		},
		"AutoGenerateWhenNoRef": {
			cr: &v1alpha1.User{},
			// wantPw empty → assert 32-char generated
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			kube := clientfake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tc.secrets...).
				Build()

			e := &external{kube: kube}
			got, err := e.resolvePassword(context.Background(), tc.cr)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.wantPw != "" {
				assert.Equal(t, tc.wantPw, got)
				return
			}
			// Auto-generated: must be 32 chars of the allowed alphabet
			assert.Len(t, got, userhelpers.PasswordLength)
			for _, ch := range got {
				assert.True(t, isAlphanumeric(ch), "generated password contains non-alphanumeric char %q", ch)
			}
		})
	}
}

func TestDesiredMechanisms(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mechanisms []commonv1alpha1.Mechanism
		want       []string
	}{
		"ExplicitMechanisms": {
			mechanisms: []commonv1alpha1.Mechanism{mechanismSHA256},
			want:       []string{mechanismSHA256},
		},
		"DefaultMechanism": {
			mechanisms: nil,
			want:       []string{mechanismSHA512},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := userhelpers.DesiredMechanisms(tc.mechanisms)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestConnectionDetails(t *testing.T) {
	t.Parallel()

	got := userhelpers.ConnectionDetails("alice", "s3cr3t", []string{"broker1:9092", "broker2:9092"})
	assert.Equal(t, "alice", string(got["username"]))
	assert.Equal(t, "s3cr3t", string(got["password"]))
	assert.Equal(t, "broker1:9092,broker2:9092", string(got["brokers"]))
}

// helpers

func userWithPasswordRef(name, namespace, key string) *v1alpha1.User {
	return &v1alpha1.User{
		Spec: v1alpha1.UserSpec{
			ForProvider: commonv1alpha1.UserParameters{
				PasswordSecretRef: &commonv1alpha1.SecretKeySelector{
					Name:      name,
					Namespace: namespace,
					Key:       key,
				},
			},
		},
	}
}

func userWithWriteRef(name, namespace string) *v1alpha1.User {
	u := &v1alpha1.User{}
	u.Spec.WriteConnectionSecretToReference = &xpv2.SecretReference{
		Name:      name,
		Namespace: namespace,
	}
	return u
}

func secret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func isAlphanumeric(r rune) bool {
	for _, c := range userhelpers.PasswordAlphabet {
		if r == c {
			return true
		}
	}
	return false
}

func TestPopulateAtProvider(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		observed []string
		want     commonv1alpha1.UserObservation
	}{
		"TwoMechanisms": {
			observed: []string{mechanismSHA512, mechanismSHA256},
			want:     commonv1alpha1.UserObservation{Mechanisms: []string{mechanismSHA512, mechanismSHA256}},
		},
		"NoMechanisms": {
			observed: nil,
			want:     commonv1alpha1.UserObservation{Mechanisms: nil},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cr := &v1alpha1.User{}
			cr.Status.AtProvider.Mechanisms = tc.observed
			if diff := cmp.Diff(tc.want, cr.Status.AtProvider); diff != "" {
				t.Errorf("AtProvider: -want, +got:\n%s", diff)
			}
		})
	}
}

// fakeScramClient records the SCRAM alterations the controller asks for.
type fakeScramClient struct {
	upserts []kadm.UpsertSCRAM
	deletes []kadm.DeleteSCRAM
}

func (f *fakeScramClient) DescribeUserSCRAMs(_ context.Context, _ ...string) (kadm.DescribedUserSCRAMs, error) {
	return kadm.DescribedUserSCRAMs{}, nil
}

func (f *fakeScramClient) AlterUserSCRAMs(_ context.Context, del []kadm.DeleteSCRAM, upsert []kadm.UpsertSCRAM) (kadm.AlteredUserSCRAMs, error) {
	f.upserts = append(f.upserts, upsert...)
	f.deletes = append(f.deletes, del...)
	return kadm.AlteredUserSCRAMs{}, nil
}

// TestUpdateDeletesRemovedMechanisms covers the mechanisms enrolled in Kafka but
// dropped from the spec: Upsert alone leaves them in place, so Observe keeps
// reporting the resource as not up to date and the reconcile loops forever.
func TestUpdateDeletesRemovedMechanisms(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	cases := map[string]struct {
		observed    []string
		desired     []commonv1alpha1.Mechanism
		wantUpserts []string
		wantDeletes []string
	}{
		"MechanismDropped": {
			observed:    []string{mechanismSHA256, mechanismSHA512},
			desired:     []commonv1alpha1.Mechanism{mechanismSHA512},
			wantUpserts: []string{mechanismSHA512},
			wantDeletes: []string{mechanismSHA256},
		},
		"MechanismAdded": {
			observed:    []string{mechanismSHA512},
			desired:     []commonv1alpha1.Mechanism{mechanismSHA256, mechanismSHA512},
			wantUpserts: []string{mechanismSHA256, mechanismSHA512},
			wantDeletes: nil,
		},
		"MechanismSwapped": {
			observed:    []string{mechanismSHA512},
			desired:     []commonv1alpha1.Mechanism{mechanismSHA256},
			wantUpserts: []string{mechanismSHA256},
			wantDeletes: []string{mechanismSHA512},
		},
		"NoMechanismChange": {
			observed:    []string{mechanismSHA512},
			desired:     []commonv1alpha1.Mechanism{mechanismSHA512},
			wantUpserts: []string{mechanismSHA512},
			wantDeletes: nil,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cr := userWithPasswordRef("my-secret", "default", "password")
			meta.SetExternalName(cr, "alice")
			cr.Spec.ForProvider.Mechanisms = tc.desired
			cr.Status.AtProvider.Mechanisms = tc.observed

			kube := clientfake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(secret("my-secret", "default", map[string][]byte{"password": []byte("s3cr3t")})).
				Build()

			cl := &fakeScramClient{}
			e := &external{kafkaClient: cl, kube: kube, brokers: []string{"broker:9092"}}

			_, err := e.Update(context.Background(), cr)
			require.NoError(t, err)

			assert.ElementsMatch(t, tc.wantUpserts, mechNames(cl.upserts, nil))
			assert.ElementsMatch(t, tc.wantDeletes, mechNames(nil, cl.deletes))
		})
	}
}

// mechNames flattens whichever of the two alteration slices is non-nil into
// mechanism names.
func mechNames(upserts []kadm.UpsertSCRAM, deletes []kadm.DeleteSCRAM) []string {
	names := make([]string, 0, len(upserts)+len(deletes))
	for _, u := range upserts {
		names = append(names, u.Mechanism.String())
	}
	for _, d := range deletes {
		names = append(names, d.Mechanism.String())
	}
	return names
}

// TestConnectMalformedCredentials pins the credential parse failure: swallowing
// it left brokers empty, and the connection Secret then advertised brokers=""
// with no error anywhere.
func TestConnectMalformedCredentials(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		creds       []byte
		wantErr     bool
		wantBrokers []string
	}{
		"ValidCredentials": {
			creds:       []byte(`{"brokers":["broker:9092"]}`),
			wantBrokers: []string{"broker:9092"},
		},
		"MalformedCredentials": {
			creds:   []byte(`not json`),
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, apis.AddToScheme(scheme))

			cr := &v1alpha1.User{
				TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.UserKind},
				ObjectMeta: metav1.ObjectMeta{Name: "alice", UID: "user-uid"},
			}
			cr.Spec.ProviderConfigReference = &xpv2.Reference{Name: "pc"}

			pc := &apisv1alpha1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "pc"},
				Spec: apisv1alpha1.ProviderConfigSpec{
					Credentials: apisv1alpha1.ProviderCredentials{
						Source: xpv2.CredentialsSourceSecret,
						CommonCredentialSelectors: xpv2.CommonCredentialSelectors{
							SecretRef: &xpv2.SecretKeySelector{
								SecretReference: xpv2.SecretReference{Name: "creds", Namespace: "default"},
								Key:             "credentials",
							},
						},
					},
				},
			}

			kube := clientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(pc, secret("creds", "default", map[string][]byte{"credentials": tc.creds})).
				Build()

			c := &connector{
				cache: &kafka.ClientCache{},
				kube:  kube,
				usage: resource.NewLegacyProviderConfigUsageTracker(kube, &apisv1alpha1.ProviderConfigUsage{}),
				newServiceFn: func(_ context.Context, _ []byte, _ client.Client) (*kadm.Client, error) {
					return &kadm.Client{}, nil
				},
			}

			got, err := c.Connect(context.Background(), cr)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantBrokers, got.(*external).brokers)
		})
	}
}
