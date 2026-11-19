package connector

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	apis "github.com/crossplane-contrib/provider-kafka/apis/cluster"
	"github.com/crossplane-contrib/provider-kafka/apis/cluster/connector/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/cluster/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/connect"
	connhelpers "github.com/crossplane-contrib/provider-kafka/internal/controller/connector"
)

func TestConnect(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		creds   string
		wantErr string
	}{
		"WithConnectBlock": {creds: `{"brokers":["b:9092"],"connect":{"url":"http://connect:8083"}}`},
		"ConnectOnly":      {creds: `{"connect":{"url":"http://connect:8083"}}`},
		"NoConnectBlock":   {creds: `{"brokers":["b:9092"]}`, wantErr: connhelpers.ErrNewClient},
		"Malformed":        {creds: `not json`, wantErr: connhelpers.ErrParseCreds},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, cr := newConnector(t, []byte(tc.creds))

			ext, err := c.Connect(context.Background(), cr)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.NoError(t, ext.Disconnect(context.Background()))
		})
	}
}

// newConnector returns a connector and a Connector whose ProviderConfig
// credentials are creds.
func newConnector(t *testing.T, creds []byte) (*connector, *v1alpha1.Connector) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apis.AddToScheme(scheme))

	cr := &v1alpha1.Connector{
		APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.ConnectorKind,
		Name: "my-connector", UID: "connector-uid",
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
	sec := &corev1.Secret{
		Name: "creds", Namespace: "default",
		Data: map[string][]byte{"credentials": creds},
	}
	kube := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(pc, sec).Build()

	return &connector{
		kube:  kube,
		usage: resource.NewLegacyProviderConfigUsageTracker(kube, &apisv1alpha1.ProviderConfigUsage{}),
		cache: &kafka.Cache[*connect.Client]{},
	}, cr
}
