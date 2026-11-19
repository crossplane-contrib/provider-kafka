package topic

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
	"github.com/crossplane-contrib/provider-kafka/apis/cluster/topic/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/cluster/v1alpha1"
	ctrlcommon "github.com/crossplane-contrib/provider-kafka/internal/controller/common"
	"github.com/crossplane-contrib/provider-kafka/internal/controller/common/commontest"
)

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

// newConnector returns a connector backed by cache, and a Topic whose ProviderConfig credentials are creds.
func newConnector(t *testing.T, cache ctrlcommon.ClientCache, creds []byte) (*connector, *v1alpha1.Topic) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apis.AddToScheme(scheme))

	cr := &v1alpha1.Topic{
		APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.TopicKind,
		Name: "topic", UID: "topic-uid",
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
	secret := &corev1.Secret{
		Name: "creds", Namespace: "default",
		Data: map[string][]byte{"credentials": creds},
	}
	kube := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(pc, secret).Build()

	return &connector{
		cache: cache,
		kube:  kube,
		usage: resource.NewLegacyProviderConfigUsageTracker(kube, &apisv1alpha1.ProviderConfigUsage{}),
		newServiceFn: func(context.Context, []byte, client.Client) (*kadm.Client, error) {
			return &kadm.Client{}, nil
		},
	}, cr
}
