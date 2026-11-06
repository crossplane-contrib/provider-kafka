---
status: in_progress
approved_at: "2026-11-05T23:49:26.240Z"
updated: "2026-11-05T23:50:14.434Z"
started_at: "2026-11-05T23:50:14.434Z"
---
# Plan: Kafka Connect Connector managed resource

**Created:** 2026-11-06 | **Status:** Draft | **Effort:** M | **Branch:** 36-kafka-connect-and-connectors

## Summary

Add a `Connector` managed resource, cluster and namespaced scope, that manages connectors on an existing self-managed Kafka Connect cluster through the Connect REST API. Deploying Connect clusters and MSK Connect are out of scope (provider-upjet-aws has a `kafkaconnect` group for MSK Connect). Closes #36.

## Architecture Context

- Reconcile flow: `Connect()` reads the ProviderConfig secret JSON into `kafka.Config`, then `Observe` / `Create` / `Update` / `Delete` call the external API. Template: `internal/controller/cluster/user/user.go` and `internal/controller/namespaced/user/user.go`.
- Connector does not use `kadm` or `kafka.ClientCache`. It builds a plain `net/http` client from a new optional `connect` block in the same secret JSON.
- `NewAdminClient` fails without `brokers`, but the connector controller never calls it, so a secret with only a `connect` block works for connectors.
- Shared types live in `apis/v1alpha1/*_types.go`. Each scope has its own group package (`apis/{cluster,namespaced}/user/v1alpha1`) registered in `apis/{cluster,namespaced}/kafka.go`.
- Shared controller logic lives in `internal/controller/user/helpers.go`. Connector follows the same pattern in `internal/controller/connector/helpers.go` to keep both scoped controllers thin.
- Controllers are registered in `internal/controller/{cluster,namespaced}/kafka.go` (both `Setup` and `SetupGated` lists).
- `make test` already installs Strimzi in kind (`Makefile` target `kind-kafka-setup`, `cluster/local/kafka-cluster.yaml`), so the e2e Connect worker is one more Strimzi `KafkaConnect` CR.
- ADR: Connect endpoint goes in the existing ProviderConfig secret JSON — one secret per Kafka environment and no new ProviderConfig kind — rejected a separate `ConnectProviderConfig` kind.

## Research Findings

- Connector lives in the base groups `kafka.crossplane.io` (cluster) and `kafka.m.crossplane.io` (namespaced), alongside ProviderConfig, so its full name is `connector.kafka.crossplane.io`. Topic, ACL and User still use per-resource groups.
- Verified in Apache Kafka source (`AbstractHerder.connectorInfo`): `GET /connectors/{name}/config` returns the raw stored config. Placeholders like `${file:...}` stay unresolved, password fields are not masked, and no defaults are added. `PUT .../config` adds `name` when absent (`ConnectorsResource.checkAndPutConnectorConfigName`). Plain map equality of desired config plus `name` against observed config is therefore correct, and it also detects removed keys.
- `PUT /connectors/{name}/config` is an idempotent upsert: 201 when created, 200 when updated. The body is the flat config map and every value is a JSON string (`tasks.max: "2"`).
- `GET /connectors/{name}/status` returns `connector.state` and `tasks[].state` in RUNNING, PAUSED, STOPPED, FAILED or UNASSIGNED. `trace` is present only on FAILED.
- `PUT .../pause`, `.../resume` and `.../stop` (Kafka 3.5+) return 202 and are asynchronous. `DELETE` returns 204, or 404 when the connector is missing.
- 409 means a rebalance is in progress. Return the error and let the managed reconciler requeue with backoff. No retry loop.
- `initial_state` exists only on `POST /connectors` (Kafka 3.7+), so Create uses PUT followed by a state call.
- Strimzi strips `connector.class`, `tasks.max` and `name` from `spec.config`. Here a CEL rule rejects them instead, because silently dropping user input hides mistakes.
- Built-in config providers: `FileConfigProvider`, `DirectoryConfigProvider`, and `EnvVarConfigProvider` (3.5+). Since 3.8 all three are registered automatically.
- `FileStreamSource` is no longer on the default plugin path (KAFKA-13748). Mirror connectors are, so e2e uses `MirrorHeartbeatConnector`.
- 36 test call sites use `configureClientCertificate(ctx, Config, ...)` and `configureTLSAdvanced`. Add a wrapper instead of changing those signatures.

## Security Considerations

- `GET .../config` returns secrets in plaintext. Never copy observed config into `status`, events, logs or error messages.
- Connector config in the spec is stored in etcd as plaintext. The docs and examples must point users to Connect config providers for credentials.
- The basic auth password sits in the ProviderConfig secret, next to the existing SASL password. Same trust level.
- TLS reuses the existing `TLS` struct (custom CA, mTLS, `insecureSkipVerify`).
- Cap response bodies with `io.LimitReader` (1 MiB) so a misbehaving endpoint cannot exhaust provider memory.
- Only cluster admins set the Connect URL, through the ProviderConfig secret. No SSRF exposure beyond what `brokers` already has.

## Performance Considerations

- Each poll makes 2 GETs per connector (config and status) at the default 1m poll interval. This is negligible.
- A new `http.Client` per reconcile means a new TCP/TLS handshake each time. `Disconnect` calls `CloseIdleConnections` so connections do not leak. `ponytail:` add a client cache keyed by the secret digest if reconcile volume makes handshakes matter.
- `http.Client.Timeout` of 30s, plus the reconcile context, bounds hung calls.

## Open Questions

### Important (P2 - Affects implementation)
1. MirrorHeartbeatConnector may need `target.cluster.bootstrap.servers` and SCRAM settings to reach RUNNING. Default: the integration test asserts only the lifecycle (create, read config, read status, pause, delete), never RUNNING. Readiness logic is covered by `httptest` unit tests.

### Unresolved / waiting on signal
- Auto-restart of FAILED connectors and tasks, as Strimzi does. Waiting on user demand. Revisit when someone asks.
- Secret refs inside connector config. Waiting on answers to the issue comment.
- Confluent Platform may mask password values on GET; Apache Kafka does not. Revisit if a user reports permanent drift.

## Steps

### Step 1: Connect config and reusable TLS builder
- **Test:** `internal/clients/kafka/client_test.go`. `NewTLSConfig` returns nil for nil input, sets `InsecureSkipVerify`, loads a CA from file, and rejects both client certificate sources set together.
- **Implement:** in `internal/clients/kafka/config.go`, add a `Connect` field to `Config`. In `client.go`, add `NewTLSConfig` and use it in `NewAdminClient` in place of the inline block.
- **Code:**
```go
// Config
Connect *Connect `json:"connect,omitempty"`

// Connect is the Kafka Connect REST endpoint used by Connector resources.
type Connect struct {
	URL       string     `json:"url"`
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
	TLS       *TLS       `json:"tls,omitempty"`
}

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"` //nolint:gosec
}

// NewTLSConfig builds a *tls.Config from t. It returns nil when t is nil.
func NewTLSConfig(ctx context.Context, t *TLS, kube client.Client) (*tls.Config, error) {
	if t == nil {
		return nil, nil
	}
	tc := &tls.Config{InsecureSkipVerify: t.InsecureSkipVerify} //nolint:gosec
	if err := configureClientCertificate(ctx, Config{TLS: t}, kube, tc); err != nil {
		return nil, err
	}
	return tc, configureTLSAdvanced(t, tc)
}
```
- **Validation:** `go test ./internal/clients/kafka/ -run 'TestNewTLSConfig|TestConfigure'`

### Step 2: Connect REST client
- **Test:** `internal/clients/kafka/connect/connect_test.go`. Table tests against `httptest.Server`:
  - `GetConfig` returns exists=false on 404.
  - `PutConfig` sends the flat map.
  - `Delete` treats 404 as success.
  - `SetState` hits pause, resume or stop.
  - non-2xx responses surface Connect's `message` field.
  - basic auth header is set.

  Add an integration test, skipped unless `KAFKA_CONFIG` has a `connect` block.
- **Implement:** `internal/clients/kafka/connect/connect.go`, using `net/http` and `encoding/json` only.
- **Code:**
```go
type Client struct {
	http       *http.Client
	base       string
	user, pass string
}

func New(ctx context.Context, c *kafka.Connect, kube client.Client) (*Client, error) {
	tc, err := kafka.NewTLSConfig(ctx, c.TLS, kube)
	if err != nil {
		return nil, err
	}
	cl := &Client{base: strings.TrimSuffix(c.URL, "/"), http: &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tc},
	}}
	if c.BasicAuth != nil {
		cl.user, cl.pass = c.BasicAuth.Username, c.BasicAuth.Password
	}
	return cl, nil
}

type Status struct {
	Connector struct {
		State    string `json:"state"`
		WorkerID string `json:"worker_id"`
	} `json:"connector"`
	Tasks []struct {
		ID       int32  `json:"id"`
		State    string `json:"state"`
		WorkerID string `json:"worker_id"`
	} `json:"tasks"`
}

func (c *Client) GetConfig(ctx context.Context, name string) (map[string]string, bool, error)
func (c *Client) PutConfig(ctx context.Context, name string, cfg map[string]string) error
func (c *Client) GetStatus(ctx context.Context, name string) (*Status, error)
func (c *Client) SetState(ctx context.Context, name, state string) error // "pause" | "resume" | "stop"
func (c *Client) Delete(ctx context.Context, name string) error
func (c *Client) Close() { c.http.CloseIdleConnections() }

// do sends the request, decodes into out when non-nil, and returns
// (status, error). Bodies are read through io.LimitReader(resp.Body, 1<<20).
func (c *Client) do(ctx context.Context, method, path string, in, out any) (int, error)
```
- **Constraint:** escape the connector name in every path with `url.PathEscape`. Never include request bodies in errors.
- **Depends on:** Step 1
- **Validation:** `go test ./internal/clients/kafka/connect/`

### Step 3: API types (both scopes)
- **Implement:**
  - `apis/v1alpha1/connector_types.go` for shared params and observation.
  - `apis/{cluster,namespaced}/connector/v1alpha1/{doc.go,groupversion_info.go,connector_types.go}`, mirroring `user/v1alpha1`: `ClusterManagedResourceSpec` with scope Cluster, `ManagedResourceSpec` with scope Namespaced.
  - Register both packages in `apis/{cluster,namespaced}/kafka.go`.
- **Code:**
```go
// ConnectorParameters are the configurable fields of a Connector.
type ConnectorParameters struct {
	// Class is the connector implementation class (connector.class).
	// +kubebuilder:validation:MinLength=1
	Class string `json:"class"`
	// TasksMax is the maximum number of tasks (tasks.max).
	// +kubebuilder:validation:Minimum=1
	// +optional
	TasksMax *int32 `json:"tasksMax,omitempty"`
	// Config holds the remaining connector properties. Use Connect config
	// providers (${file:...}, ${env:...}) for credentials.
	// +kubebuilder:validation:XValidation:rule="!('connector.class' in self) && !('tasks.max' in self) && !('name' in self)",message="set connector.class, tasks.max and name through class, tasksMax and the external name"
	// +optional
	Config map[string]string `json:"config,omitempty"`
	// State is the desired lifecycle state.
	// +kubebuilder:validation:Enum=running;paused;stopped
	// +kubebuilder:default=running
	// +optional
	State string `json:"state,omitempty"`
}

type ConnectorObservation struct {
	State    string       `json:"state,omitempty"`
	WorkerID string       `json:"workerId,omitempty"`
	Tasks    []TaskStatus `json:"tasks,omitempty"`
}

type TaskStatus struct {
	ID       int32  `json:"id"`
	State    string `json:"state"`
	WorkerID string `json:"workerId,omitempty"`
}
```
- **Validation:** `make generate && go build ./...`. The CRDs `kafka.crossplane.io_connectors.yaml` and `kafka.m.crossplane.io_connectors.yaml` must appear in `package/crds/`.

### Step 4: Desired state, drift and readiness helpers
- **Test:** `internal/controller/connector/helpers_test.go`, table tests:
  - `Desired` merges class, tasksMax, config and name.
  - `UpToDate` is false when a key is added, changed or removed.
  - `StateUpToDate` ignores FAILED and UNASSIGNED, so the controller does not loop on resume.
  - `Available` requires the connector and all tasks to be in the desired state.
- **Implement:** `internal/controller/connector/helpers.go`
- **Code:**
```go
var stateNames = map[string]string{"running": "RUNNING", "paused": "PAUSED", "stopped": "STOPPED"}
var stateCalls = map[string]string{"running": "resume", "paused": "pause", "stopped": "stop"}

func Desired(p v1alpha1.ConnectorParameters, name string) map[string]string {
	out := maps.Clone(p.Config)
	if out == nil {
		out = map[string]string{}
	}
	out["connector.class"] = p.Class
	out["name"] = name
	if p.TasksMax != nil {
		out["tasks.max"] = strconv.Itoa(int(*p.TasksMax))
	}
	return out
}

func UpToDate(desired, observed map[string]string) bool { return maps.Equal(desired, observed) }

// StateUpToDate reports drift only between the settable states.
// FAILED and UNASSIGNED are not fixed by pause, resume or stop.
func StateUpToDate(desired string, s *connect.Status) bool {
	switch s.Connector.State {
	case "RUNNING", "PAUSED", "STOPPED":
		return s.Connector.State == stateNames[desired]
	}
	return true
}
```
- **Validation:** `go test ./internal/controller/connector/`

### Step 5: Cluster-scoped controller
- **Test:** `internal/controller/cluster/connector/connector_test.go`. The fake Connect is an `httptest.Server`, the kube client is a crossplane `test.MockClient`. Cases:
  - `Connect` errors when the secret has no `connect` block.
  - `Observe` returns not-exists on 404; returns not-up-to-date on config drift or state drift; sets Available and `atProvider` when everything is RUNNING.
  - `Create` PUTs and then pauses when desired is paused.
  - `Update` skips the PUT when only the state differs.
  - `Delete` succeeds on 404.
- **Implement:** `internal/controller/cluster/connector/connector.go`, copied from the cluster user controller with `ClientCache` removed. Add `connector.Setup` and `connector.SetupGated` to `internal/controller/cluster/kafka.go`.
- **Code:**
```go
func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr := mg.(*v1alpha1.Connector)
	name := meta.GetExternalName(cr)
	observed, exists, err := c.client.GetConfig(ctx, name)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", connhelpers.ErrObserve, err)
	}
	if !exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	st, err := c.client.GetStatus(ctx, name)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", connhelpers.ErrObserve, err)
	}
	cr.Status.AtProvider = connhelpers.Observation(st)
	cr.Status.SetConditions(connhelpers.Condition(cr.Spec.ForProvider.State, st))
	desired := connhelpers.Desired(cr.Spec.ForProvider, name)
	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: connhelpers.UpToDate(desired, observed) && connhelpers.StateUpToDate(cr.Spec.ForProvider.State, st),
	}, nil
}

func (c *external) Disconnect(context.Context) error { c.client.Close(); return nil }
```
- **Constraint:** the external name is the connector name, defaulting to `metadata.name`. `Condition` returns `xpv2.Unavailable()` with the first non-matching connector or task state in the message, and never a `trace`.
- **Depends on:** Steps 2, 3, 4
- **Validation:** `go test ./internal/controller/cluster/connector/`

### Step 6: Namespaced controller
- **Test:** `internal/controller/namespaced/connector/connector_test.go`. Same cases as Step 5, plus `Connect` resolving both `ProviderConfig` and `ClusterProviderConfig`.
- **Implement:** `internal/controller/namespaced/connector/connector.go`, using the ProviderConfig / ClusterProviderConfig switch from `namespaced/user/user.go:132-155`. Register it in `internal/controller/namespaced/kafka.go`.
- **Constraint:** two namespaces whose Connectors share a name and a Connect cluster write to the same connector. Topic has the same behaviour. Document it in the README instead of adding a prefix.
- **Depends on:** Step 5
- **Validation:** `go test ./internal/controller/namespaced/connector/`

### Step 7: e2e Connect worker
- **Test:** the integration test from Step 2 runs under `make test`. It creates `MirrorHeartbeatConnector` (`source.cluster.alias: a`, `target.cluster.alias: b`), reads the config back, reads the status, pauses, and deletes.
- **Implement:**
  - In `cluster/local/kafka-connect.yaml`, add a Strimzi `KafkaConnect` named `connect`. It uses bootstrap `dev-kafka-bootstrap:9092`, SCRAM auth as `user`, and replication factors of 1. Add a NodePort `Service` on port 30083 that selects the Connect pods.
  - In `Makefile` `kind-kafka-setup`, apply that file, wait for `kafkaconnect/connect`, and add a `connect` block to `kc.json`.
- **Code:**
```make
	@$(KUBECTL) apply -f cluster/local/kafka-connect.yaml
	@$(KUBECTL) wait --for=condition=ready -n kafka-cluster kafkaconnect/connect --timeout=300s
	# kc.json gains:
	#   "connect": { "url": "http://$${KIND_NODE_IP}:30083" }
```
- **Depends on:** Step 2
- **Validation:** `make test`

## Acceptance Criteria

- [ ] `kubectl apply` of `examples/{cluster,namespaced}/connector/connector.yaml` creates the connector on the Connect cluster, and the resource becomes Ready.
- [ ] A spec config change, or a manual change on the Connect side, is reverted on the next poll.
- [ ] `state: paused` and `state: stopped` move the connector into that state, and `running` resumes it.
- [ ] Deleting the resource deletes the connector. A connector that is already gone does not block deletion.
- [ ] The CRD rejects `connector.class`, `tasks.max` or `name` inside `config`.
- [ ] Topic, ACL and User behave unchanged when the secret has no `connect` block.
- [ ] All tests pass (`make review`, `make test`).

## Checklist (non-TDD cleanup)

- [ ] Lint clean (`make lint`)
- [ ] `examples/cluster/connector/connector.yaml` and `examples/namespaced/connector/connector.yaml`
- [ ] README: a "Managing Kafka Connect connectors" section covering the secret `connect` block, the config provider advice, and the namespace name-collision note
- [ ] Reply on #36 with the scope comment, naming provider-upjet-aws for MSK Connect
