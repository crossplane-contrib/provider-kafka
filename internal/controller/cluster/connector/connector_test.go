package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource/fake"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	apis "github.com/crossplane-contrib/provider-kafka/apis/cluster"
	"github.com/crossplane-contrib/provider-kafka/apis/cluster/connector/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/cluster/v1alpha1"
	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/connect"
	connhelpers "github.com/crossplane-contrib/provider-kafka/internal/controller/connector"
)

const (
	connName = "my-connector"
	running  = "RUNNING"
)

// fakeConnect is an in-memory Kafka Connect REST API for one connector.
type fakeConnect struct {
	mu     sync.Mutex
	config map[string]string // nil means the connector does not exist
	state  string
	tasks  []string
	calls  []string

	noStatus bool // status not written yet, as right after creation
}

func (f *fakeConnect) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := strings.TrimPrefix(r.URL.Path, "/connectors/"+connName)
	if r.Method != http.MethodGet {
		f.calls = append(f.calls, r.Method+" "+path)
	}
	notFound := func() {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error_code":404,"message":"Connector not found"}`))
	}

	switch {
	case r.Method == http.MethodGet && path == "/config":
		if f.config == nil {
			notFound()
			return
		}
		writeJSON(w, f.config)
	case r.Method == http.MethodGet && path == "/status":
		if f.config == nil || f.noStatus {
			notFound()
			return
		}
		st := connect.Status{}
		st.Connector.State = f.state
		for i, s := range f.tasks {
			st.Tasks = append(st.Tasks, connect.TaskStatus{ID: int32(i), State: s})
		}
		writeJSON(w, st)
	case r.Method == http.MethodPut && path == "/config":
		var cfg map[string]string
		_ = json.NewDecoder(r.Body).Decode(&cfg)
		cfg["name"] = connName
		f.config = cfg
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPut:
		w.WriteHeader(http.StatusAccepted)
	case r.Method == http.MethodDelete:
		if f.config == nil {
			notFound()
			return
		}
		f.config = nil
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(b)
}

func newExternal(t *testing.T, f *fakeConnect) *external {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	cl, err := connect.New(context.Background(), &kafka.Connect{URL: srv.URL}, nil)
	require.NoError(t, err)
	return &external{client: cl}
}

func newConnectorCR(params commonv1alpha1.ConnectorParameters) *v1alpha1.Connector {
	cr := &v1alpha1.Connector{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: v1alpha1.ConnectorKind},
		ObjectMeta: metav1.ObjectMeta{Name: connName, UID: "connector-uid"},
	}
	cr.Spec.ForProvider = params
	meta.SetExternalName(cr, connName)
	return cr
}

var heartbeat = commonv1alpha1.ConnectorParameters{
	Class:  "org.apache.kafka.connect.mirror.MirrorHeartbeatConnector",
	Config: map[string]string{"source.cluster.alias": "a"},
}

func storedHeartbeat() map[string]string {
	return connhelpers.Desired(heartbeat, connName)
}

func TestWrongType(t *testing.T) {
	t.Parallel()

	e := &external{}
	ctx := context.Background()
	_, err := e.Observe(ctx, &fake.Managed{})
	require.ErrorContains(t, err, connhelpers.ErrNotConnector)
	_, err = e.Create(ctx, &fake.Managed{})
	require.ErrorContains(t, err, connhelpers.ErrNotConnector)
	_, err = e.Update(ctx, &fake.Managed{})
	require.ErrorContains(t, err, connhelpers.ErrNotConnector)
	_, err = e.Delete(ctx, &fake.Managed{})
	require.ErrorContains(t, err, connhelpers.ErrNotConnector)
}

func TestObserve(t *testing.T) {
	t.Parallel()

	paused := heartbeat
	paused.State = commonv1alpha1.ConnectorStatePaused

	changed := heartbeat
	changed.Config = map[string]string{"source.cluster.alias": "z"}

	cases := map[string]struct {
		params        commonv1alpha1.ConnectorParameters
		fake          *fakeConnect
		wantExists    bool
		wantUpToDate  bool
		wantReady     corev1.ConditionStatus
		wantStateSeen string
	}{
		"NotFound": {
			params: heartbeat,
			fake:   &fakeConnect{},
		},
		"UpToDateAndRunning": {
			params:        heartbeat,
			fake:          &fakeConnect{config: storedHeartbeat(), state: running, tasks: []string{running}},
			wantExists:    true,
			wantUpToDate:  true,
			wantReady:     corev1.ConditionTrue,
			wantStateSeen: running,
		},
		"ConfigDrift": {
			params:        changed,
			fake:          &fakeConnect{config: storedHeartbeat(), state: running},
			wantExists:    true,
			wantReady:     corev1.ConditionTrue,
			wantStateSeen: running,
		},
		"StateDrift": {
			params:        paused,
			fake:          &fakeConnect{config: storedHeartbeat(), state: running},
			wantExists:    true,
			wantReady:     corev1.ConditionFalse,
			wantStateSeen: running,
		},
		"StatusNotYetWritten": {
			params:       heartbeat,
			fake:         &fakeConnect{config: storedHeartbeat(), noStatus: true},
			wantExists:   true,
			wantUpToDate: true,
			wantReady:    corev1.ConditionFalse,
		},
		"TaskFailedIsNotDrift": {
			params:        heartbeat,
			fake:          &fakeConnect{config: storedHeartbeat(), state: running, tasks: []string{"FAILED"}},
			wantExists:    true,
			wantUpToDate:  true,
			wantReady:     corev1.ConditionFalse,
			wantStateSeen: running,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newExternal(t, tc.fake)
			cr := newConnectorCR(tc.params)

			obs, err := e.Observe(context.Background(), cr)
			require.NoError(t, err)
			assert.Equal(t, tc.wantExists, obs.ResourceExists)
			assert.Equal(t, tc.wantUpToDate, obs.ResourceUpToDate)
			if !tc.wantExists {
				return
			}
			assert.Equal(t, tc.wantReady, cr.Status.GetCondition(xpv2.TypeReady).Status)
			assert.Equal(t, tc.wantStateSeen, cr.Status.AtProvider.State)
		})
	}
}

func TestCreate(t *testing.T) {
	t.Parallel()

	paused := heartbeat
	paused.State = commonv1alpha1.ConnectorStatePaused

	cases := map[string]struct {
		params    commonv1alpha1.ConnectorParameters
		wantCalls []string
	}{
		"Running": {
			params:    heartbeat,
			wantCalls: []string{"PUT /config"},
		},
		"Paused": {
			params:    paused,
			wantCalls: []string{"PUT /config", "PUT /pause"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := &fakeConnect{}
			e := newExternal(t, f)

			_, err := e.Create(context.Background(), newConnectorCR(tc.params))
			require.NoError(t, err)
			assert.Equal(t, tc.wantCalls, f.calls)
			assert.Equal(t, connhelpers.Desired(tc.params, connName), f.config)
		})
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	stopped := heartbeat
	stopped.State = commonv1alpha1.ConnectorStateStopped

	changed := heartbeat
	changed.Config = map[string]string{"source.cluster.alias": "z"}

	cases := map[string]struct {
		params    commonv1alpha1.ConnectorParameters
		fake      *fakeConnect
		wantCalls []string
	}{
		"ConfigOnly": {
			params:    changed,
			fake:      &fakeConnect{config: storedHeartbeat(), state: running},
			wantCalls: []string{"PUT /config"},
		},
		"StateOnly": {
			params:    stopped,
			fake:      &fakeConnect{config: storedHeartbeat(), state: running},
			wantCalls: []string{"PUT /stop"},
		},
		"Resume": {
			params:    heartbeat,
			fake:      &fakeConnect{config: storedHeartbeat(), state: "PAUSED"},
			wantCalls: []string{"PUT /resume"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newExternal(t, tc.fake)
			cr := newConnectorCR(tc.params)

			// The managed reconciler always observes before it updates.
			_, err := e.Observe(context.Background(), cr)
			require.NoError(t, err)
			_, err = e.Update(context.Background(), cr)
			require.NoError(t, err)
			assert.Equal(t, tc.wantCalls, tc.fake.calls)
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	for name, f := range map[string]*fakeConnect{
		"Exists":  {config: storedHeartbeat(), state: running},
		"Missing": {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := newExternal(t, f)
			cr := newConnectorCR(heartbeat)

			_, err := e.Delete(context.Background(), cr)
			require.NoError(t, err)
			assert.Equal(t, []string{"DELETE "}, f.calls)
			assert.Nil(t, f.config)
			assert.Equal(t, xpv2.ReasonDeleting, cr.Status.GetCondition(xpv2.TypeReady).Reason)
		})
	}
}

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

	cr := newConnectorCR(heartbeat)
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
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "default"},
		Data:       map[string][]byte{"credentials": creds},
	}
	kube := clientfake.NewClientBuilder().WithScheme(scheme).WithObjects(pc, sec).Build()

	return &connector{
		kube:  kube,
		usage: resource.NewLegacyProviderConfigUsageTracker(kube, &apisv1alpha1.ProviderConfigUsage{}),
	}, cr
}
