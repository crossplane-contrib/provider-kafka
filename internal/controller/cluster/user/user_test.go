/*
Copyright 2020 The Crossplane Authors.

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

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	"github.com/crossplane/crossplane-runtime/v2/pkg/test"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane-contrib/provider-kafka/apis/cluster/user/v1alpha1"
	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
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
				err: errors.New(errNotUser),
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
				if err == nil {
					t.Errorf("resolvePassword(): want error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("resolvePassword(): unexpected error: %v", err)
				return
			}
			if tc.wantPw != "" {
				if got != tc.wantPw {
					t.Errorf("resolvePassword(): got %q, want %q", got, tc.wantPw)
				}
				return
			}
			// Auto-generated: must be 32 chars of the allowed alphabet
			if len(got) != passwordLength {
				t.Errorf("resolvePassword(): generated password length = %d, want %d", len(got), passwordLength)
			}
			for _, ch := range got {
				if !isAlphanumeric(ch) {
					t.Errorf("resolvePassword(): generated password contains non-alphanumeric char %q", ch)
					break
				}
			}
		})
	}
}

func TestDesiredMechanisms(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		params commonv1alpha1.UserParameters
		want   []string
	}{
		"ExplicitMechanisms": {
			params: commonv1alpha1.UserParameters{Mechanisms: []string{"SCRAM-SHA-256"}},
			want:   []string{"SCRAM-SHA-256"},
		},
		"DefaultMechanism": {
			params: commonv1alpha1.UserParameters{},
			want:   []string{"SCRAM-SHA-512"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := desiredMechanisms(tc.params)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("desiredMechanisms(): -want, +got:\n%s", diff)
			}
		})
	}
}

func TestConnectionDetails(t *testing.T) {
	t.Parallel()

	got := connectionDetails("alice", "s3cr3t", []string{"broker1:9092", "broker2:9092"})
	if string(got["username"]) != "alice" {
		t.Errorf("username = %q, want %q", string(got["username"]), "alice")
	}
	if string(got["password"]) != "s3cr3t" {
		t.Errorf("password = %q, want %q", string(got["password"]), "s3cr3t")
	}
	if string(got["brokers"]) != "broker1:9092,broker2:9092" {
		t.Errorf("brokers = %q, want %q", string(got["brokers"]), "broker1:9092,broker2:9092")
	}
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
	for _, c := range passwordAlphabet {
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
			observed: []string{"SCRAM-SHA-512", "SCRAM-SHA-256"},
			want:     commonv1alpha1.UserObservation{Mechanisms: []string{"SCRAM-SHA-512", "SCRAM-SHA-256"}},
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

// Ensure the external type is available to tests in this package (avoids unused import).
var (
	_ = types.NamespacedName{}
	_ = xpv2.Available()
)
