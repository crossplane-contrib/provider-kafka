package connector

import (
	"context"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	apis "github.com/crossplane-contrib/provider-kafka/apis/namespaced"
	"github.com/crossplane-contrib/provider-kafka/apis/namespaced/connector/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/namespaced/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/connect"
	connhelpers "github.com/crossplane-contrib/provider-kafka/internal/controller/connector"
)

func TestConnect(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		kind    string
		creds   string
		wantErr string
	}{
		"ProviderConfig":        {kind: "ProviderConfig", creds: `{"connect":{"url":"http://connect:8083"}}`},
		"ClusterProviderConfig": {kind: "ClusterProviderConfig", creds: `{"connect":{"url":"http://connect:8083"}}`},
		"UnsupportedKind":       {kind: "Bogus", creds: `{"connect":{"url":"http://connect:8083"}}`, wantErr: "unsupported provider config kind"},
		"NoConnectBlock":        {kind: "ProviderConfig", creds: `{"brokers":["b:9092"]}`, wantErr: connhelpers.ErrNewClient},
		"Malformed":             {kind: "ProviderConfig", creds: `not json`, wantErr: connhelpers.ErrParseCreds},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, cr := newConnector(t, tc.kind, []byte(tc.creds))

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

// newConnector returns a connector and a Connector that references a
// provider config of the given kind whose credentials are creds.
func newConnector(t *testing.T, kind string, creds []byte) (*connector, *v1alpha1.Connector) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apis.AddToScheme(scheme))

	cr := &v1alpha1.Connector{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.ConnectorKind},
		ObjectMeta: metav1.ObjectMeta{Name: "my-connector", Namespace: "team-a", UID: "connector-uid"},
	}
	cr.Spec.ProviderConfigReference = &xpv2.ProviderConfigReference{Name: "pc", Kind: kind}

	spec := apisv1alpha1.ProviderConfigSpec{
		Credentials: apisv1alpha1.ProviderCredentials{
			Source: xpv2.CredentialsSourceSecret,
			CommonCredentialSelectors: xpv2.CommonCredentialSelectors{
				SecretRef: &xpv2.SecretKeySelector{
					SecretReference: xpv2.SecretReference{Name: "creds", Namespace: cr.Namespace},
					Key:             "credentials",
				},
			},
		},
	}
	pc := &apisv1alpha1.ProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: "pc", Namespace: cr.Namespace}, Spec: spec}
	cpc := &apisv1alpha1.ClusterProviderConfig{ObjectMeta: metav1.ObjectMeta{Name: "pc"}, Spec: spec}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: cr.Namespace},
		Data:       map[string][]byte{"credentials": creds},
	}
	kube := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(pc, cpc, sec).Build()

	return &connector{
		kube:  kube,
		usage: resource.NewProviderConfigUsageTracker(kube, &apisv1alpha1.ProviderConfigUsage{}),
		cache: &kafka.Cache[*connect.Client]{},
	}, cr
}
