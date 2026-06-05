# provider-kafka

Crossplane provider for managing Apache Kafka resources (topics, ACLs) as Kubernetes custom resources.

## Module

`github.com/crossplane-contrib/provider-kafka` — Go 1.26+

## Project Structure

```
cmd/provider/main.go          # Entrypoint — kong CLI, controller-runtime manager
apis/
  cluster/                     # Cluster-scoped CRD types (Topic, ACL, ProviderConfig)
  namespaced/                  # Namespace-scoped CRD types (Topic, ACL, ProviderConfig)
  v1alpha1/                    # Shared type definitions
internal/
  clients/kafka/
    client.go                  # NewAdminClient — builds franz-go kadm.Client from JSON config
    config.go                  # Config, SASL, TLS structs (JSON schema for provider secret)
    constants.go               # Defaults, error strings, lookup maps
    cache.go                   # Client caching layer
    topic/                     # Topic CRUD operations via kadm
    acl/                       # ACL CRUD operations via kadm
  controller/
    cluster/                   # Cluster-scoped controllers (topic, acl, config)
    namespaced/                # Namespace-scoped controllers (topic, acl, config)
  version/                     # Version var (set via -X linker flag at build)
examples/                      # Example manifests (ProviderConfig, Topic, ACL)
```

## Key Dependencies

- **franz-go** (`github.com/twmb/franz-go/pkg/kgo`, `kadm`) — Kafka client library
- **crossplane-runtime** v2 — controller framework, managed reconciler, feature flags
- **controller-runtime** — Kubernetes controller manager
- **kong** — CLI argument parsing (not cobra)
- **testify** — test assertions (`require`, `assert`)

## Code Generation

```bash
make generate        # Run go generate (CRDs, deepcopy, angryjet methodsets) + go mod tidy
```

Triggered by `go:generate` directives in `apis/generate.go`:
1. Regenerates CRD YAMLs into `package/crds/`
2. Generates deepcopy methods (`zz_generated.deepcopy.go`)
3. Generates managed resource interfaces via angryjet (`zz_generated.managed.go`, `zz_generated.managedlist.go`)
4. Generates provider config helpers (`zz_generated.pc.go`, `zz_generated.pcu.go`, `zz_generated.pculist.go`)

**Never edit these files — they are overwritten by `make generate`:**
- `apis/**/zz_generated.*.go`
- `package/crds/*.yaml`

Run `make generate` after modifying any API types in `apis/`.

## Build & Test

```bash
make review          # generate + lint + tests (full check before PR)
make build           # Build provider image
make dev             # Install CRDs, run provider out-of-cluster
make local-deploy    # Build image + deploy as Crossplane package in kind
make test            # Full integration tests (spins up kind + kafka, needs docker)
```

### Running tests locally

Unit tests that don't need Kafka:
```bash
go test ./internal/clients/kafka/ -run "TestConfigure|TestAppend|TestParseLogLevel"
```

Integration tests need `KAFKA_CONFIG` env var with JSON credentials from a running Kafka cluster.
`make test` handles this automatically (kind + helm + kafka).

```bash
go build ./...       # Quick compile check
```

## Linting

Uses golangci-lint v2 (`.golangci.yml`). Ran with `make lint`.

Local imports prefix: `github.com/crossplane-contrib/provider-kafka`

## Commit Convention

Conventional Commits: `feat(scope):`, `fix(scope):`, `chore(scope):`, `docs(scope):`

## Architecture Notes

- **Two scopes**: cluster-scoped and namespace-scoped resources. Parallel API/controller trees.
- **Provider config secret**: JSON blob with `brokers`, optional `sasl`, optional `tls`. Schema defined in `internal/clients/kafka/config.go`.
- **SASL mechanisms**: PLAIN, SCRAM-SHA-512, AWS-MSK-IAM
- **TLS**: Full config — custom CA (secret ref or file), mTLS (secret ref or file path with rotation), cipher suites, TLS version constraints, curve preferences
- **AWS MSK IAM**: Uses default AWS credential chain. Optional `roleArn` for cross-account AssumeRole. Credentials cached with configurable expiry window.
- **Client caching**: `cache.go` caches kadm.Client instances to avoid reconnecting on every reconcile
- **Log level**: `--debug` CLI flag sets franz-go kafka client to debug level (default: warn). Controlled via `kafka.LogLevel` package var, set in `main.go`.

## Testing Patterns

- Tests use `testify/require` for fatal checks, `testify/assert` for non-fatal
- Tests are `t.Parallel()` where possible
- TLS tests generate self-signed certs via `generateTestCertificate` helper
- Integration tests (`TestNewAdminClient_ValidCredentials`, `TestNewAdminClient_WrongCredentials`) need live Kafka — skip locally unless `KAFKA_CONFIG` is set
