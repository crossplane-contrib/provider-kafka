package connect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
)

// request records what the fake Connect server received.
type request struct {
	method, path string
	body         map[string]string
	user, pass   string
}

// fakeConnect starts an httptest server that records each request and
// answers with the given status and body.
func fakeConnect(t *testing.T, status int, body string) (*Client, *request) {
	t.Helper()
	got := &request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.EscapedPath()
		got.user, got.pass, _ = r.BasicAuth()
		if r.ContentLength > 0 {
			_ = json.NewDecoder(r.Body).Decode(&got.body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c, err := New(context.Background(), &kafka.Connect{URL: srv.URL + "/"}, nil)
	require.NoError(t, err)
	return c, got
}

func TestNew(t *testing.T) {
	t.Parallel()

	_, err := New(context.Background(), nil, nil)
	require.ErrorContains(t, err, errMissingConnect)

	_, err = New(context.Background(), &kafka.Connect{}, nil)
	require.ErrorContains(t, err, errMissingConnect)

	_, err = New(context.Background(), &kafka.Connect{URL: "http://x", TLS: &kafka.TLS{MinVersion: "bogus"}}, nil)
	require.Error(t, err)
}

func TestGetConfig(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status     int
		body       string
		wantExists bool
		wantCfg    map[string]string
		wantErr    string
	}{
		"Exists": {
			status:     http.StatusOK,
			body:       `{"connector.class":"X","name":"my connector"}`,
			wantExists: true,
			wantCfg:    map[string]string{"connector.class": "X", "name": "my connector"},
		},
		"NotFound": {
			status: http.StatusNotFound,
			body:   `{"error_code":404,"message":"Connector my connector not found"}`,
		},
		"ServerError": {
			status:  http.StatusInternalServerError,
			body:    `{"error_code":500,"message":"boom"}`,
			wantErr: "boom",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, got := fakeConnect(t, tc.status, tc.body)

			cfg, exists, err := c.GetConfig(context.Background(), "my connector")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantExists, exists)
			assert.Equal(t, tc.wantCfg, cfg)
			assert.Equal(t, http.MethodGet, got.method)
			assert.Equal(t, "/connectors/my%20connector/config", got.path)
		})
	}
}

func TestPutConfig(t *testing.T) {
	t.Parallel()

	c, got := fakeConnect(t, http.StatusCreated, `{"name":"a","config":{},"tasks":[]}`)
	cfg := map[string]string{"connector.class": "X", "tasks.max": "2"}

	require.NoError(t, c.PutConfig(context.Background(), "a/b", cfg))
	assert.Equal(t, http.MethodPut, got.method)
	assert.Equal(t, "/connectors/a%2Fb/config", got.path)
	assert.Equal(t, cfg, got.body)
}

func TestPutConfig_Conflict(t *testing.T) {
	t.Parallel()

	c, _ := fakeConnect(t, http.StatusConflict, `{"error_code":409,"message":"Cannot complete request momentarily due to stale configuration (typically caused by a concurrent config change)"}`)

	err := c.PutConfig(context.Background(), "a", map[string]string{})
	require.ErrorContains(t, err, "409")
	require.ErrorContains(t, err, "stale configuration")
}

func TestGetStatus(t *testing.T) {
	t.Parallel()

	c, got := fakeConnect(t, http.StatusOK, `{
		"name": "a",
		"connector": {"state": "RUNNING", "worker_id": "w1:8083"},
		"tasks": [{"id": 0, "state": "FAILED", "worker_id": "w2:8083", "trace": "java.lang.Exception"}]
	}`)

	st, err := c.GetStatus(context.Background(), "a")
	require.NoError(t, err)
	assert.Equal(t, "/connectors/a/status", got.path)
	assert.Equal(t, "RUNNING", st.Connector.State)
	assert.Equal(t, "w1:8083", st.Connector.WorkerID)
	require.Len(t, st.Tasks, 1)
	assert.Equal(t, int32(0), st.Tasks[0].ID)
	assert.Equal(t, "FAILED", st.Tasks[0].State)
	assert.Equal(t, "w2:8083", st.Tasks[0].WorkerID)
}

func TestSetState(t *testing.T) {
	t.Parallel()

	for _, action := range []string{ActionPause, ActionResume, ActionStop} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			c, got := fakeConnect(t, http.StatusAccepted, ``)

			require.NoError(t, c.SetState(context.Background(), "a", action))
			assert.Equal(t, http.MethodPut, got.method)
			assert.Equal(t, "/connectors/a/"+action, got.path)
		})
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status  int
		body    string
		wantErr string
	}{
		"Deleted":  {status: http.StatusNoContent},
		"NotFound": {status: http.StatusNotFound, body: `{"error_code":404,"message":"not found"}`},
		"Conflict": {status: http.StatusConflict, body: `{"error_code":409,"message":"rebalance"}`, wantErr: "rebalance"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, got := fakeConnect(t, tc.status, tc.body)

			err := c.Delete(context.Background(), "a")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, http.MethodDelete, got.method)
			assert.Equal(t, "/connectors/a", got.path)
		})
	}
}

func TestBasicAuth(t *testing.T) {
	t.Parallel()

	got := &request{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.user, got.pass, _ = r.BasicAuth()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c, err := New(context.Background(), &kafka.Connect{
		URL:       srv.URL,
		BasicAuth: &kafka.BasicAuth{Username: "admin", Password: "secret"},
	}, nil)
	require.NoError(t, err)

	require.NoError(t, c.Delete(context.Background(), "a"))
	assert.Equal(t, "admin", got.user)
	assert.Equal(t, "secret", got.pass)
}

// TestLifecycle_Integration runs against the Connect worker that "make test"
// deploys. It is skipped unless KAFKA_CONFIG has a connect block.
func TestLifecycle_Integration(t *testing.T) {
	var cfg kafka.Config
	_ = json.Unmarshal([]byte(os.Getenv("KAFKA_CONFIG")), &cfg)
	if cfg.Connect == nil {
		t.Skip("KAFKA_CONFIG has no connect block, skipping integration test")
	}

	ctx := context.Background()
	c, err := New(ctx, cfg.Connect, nil)
	require.NoError(t, err)
	t.Cleanup(c.Close)

	const name = "provider-kafka-it-heartbeat"
	desired := map[string]string{
		"connector.class":      "org.apache.kafka.connect.mirror.MirrorHeartbeatConnector",
		"tasks.max":            "1",
		"source.cluster.alias": "a",
		"target.cluster.alias": "b",
	}
	t.Cleanup(func() { _ = c.Delete(ctx, name) })

	require.NoError(t, c.PutConfig(ctx, name, desired))

	observed, exists, err := c.GetConfig(ctx, name)
	require.NoError(t, err)
	require.True(t, exists)
	desired["name"] = name
	assert.Equal(t, desired, observed)

	_, err = c.GetStatus(ctx, name)
	require.NoError(t, err)

	require.NoError(t, c.SetState(ctx, name, ActionPause))
	require.NoError(t, c.Delete(ctx, name))

	_, exists, err = c.GetConfig(ctx, name)
	require.NoError(t, err)
	assert.False(t, exists)
}
