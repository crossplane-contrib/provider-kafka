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
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	apis "github.com/crossplane-contrib/provider-kafka/apis/cluster"
	"github.com/crossplane-contrib/provider-kafka/apis/cluster/user/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/cluster/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/controller/common"
	"github.com/crossplane-contrib/provider-kafka/internal/controller/common/commontest"
)

func secret(name, namespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		Name:      name,
		Namespace: namespace,
		Data:      data,
	}
}

// TestConnectReleasesClient checks that Disconnect releases the client Connect acquired.
func TestConnectReleasesClient(t *testing.T) {
	t.Parallel()

	cache := &commontest.ClientCache{}
	c, cr := newConnector(t, cache, []byte(`{"brokers":["broker:9092"]}`))

	ext, err := c.Connect(context.Background(), cr)
	require.NoError(t, err)
	assert.Equal(t, 1, cache.Acquired)
	assert.Zero(t, cache.Released, "client released while connected")

	require.NoError(t, ext.Disconnect(context.Background()))
	assert.Equal(t, 1, cache.Released, "Disconnect must release the client")
}

// newConnector returns a connector backed by cache, and a User whose ProviderConfig credentials are creds.
func newConnector(t *testing.T, cache common.ClientCache, creds []byte) (*connector, *v1alpha1.User) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apis.AddToScheme(scheme))

	cr := &v1alpha1.User{
		APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.UserKind,
		Name: "alice", UID: "user-uid",
	}
	cr.Spec.ProviderConfigReference = &xpv2.Reference{Name: "pc"}

	pc := &apisv1alpha1.ProviderConfig{
		Name: "pc",
		Spec: apisv1alpha1.ProviderConfigSpec{
			Credentials: apisv1alpha1.ProviderCredentials{
				Source: xpv2.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv2.CommonCredentialSelectors{
					SecretRef: &xpv2.SecretKeySelector{
						Name: "creds", Namespace: "default",
						Key: "credentials",
					},
				},
			},
		},
	}
	kube := clientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pc, secret("creds", "default", map[string][]byte{"credentials": creds})).
		Build()

	return &connector{
		cache: cache,
		kube:  kube,
		usage: resource.NewLegacyProviderConfigUsageTracker(kube, &apisv1alpha1.ProviderConfigUsage{}),
		newServiceFn: func(context.Context, []byte, client.Client) (*kadm.Client, error) {
			return &kadm.Client{}, nil
		},
	}, cr
}
