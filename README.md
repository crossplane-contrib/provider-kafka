# provider-kafka

`provider-kafka` is a [Crossplane](https://crossplane.io/) Provider that is used to
manage [Kafka](https://kafka.apache.org/) resources.

## Usage

1. Create a provider secret containing a json like the following, see expected
  schema [here](internal/clients/kafka/config.go):

    ```json
    {
      "brokers":[
        "kafka-dev-controller-0.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092",
        "kafka-dev-controller-1.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092",
        "kafka-dev-controller-2.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092"
       ],
       "sasl":{
         "mechanism":"PLAIN",
         "username":"user1",
         "password":"<your-password>"
       }
    }
    ```

    See [providerconfig](examples/namespaced/providerconfig/) for more credential examples
    (SCRAM-SHA-512, AWS MSK IAM, TLS/mTLS).

    **TLS**: Enable TLS by adding a `tls` block. Set `insecureSkipVerify: true` to
    skip server certificate verification.

    **Custom CA**: To verify brokers signed by a private or custom CA, configure
    one of the supported CA sources:

    - `caCertificateSecretRef` - reference a Kubernetes Secret containing the CA
      certificate. By default the provider reads the `ca.crt` field; override the
      field name with `caField`.

        ```json
        "tls": {
          "caCertificateSecretRef": {
            "name": "kafka-ca",
            "namespace": "kafka-cluster",
            "caField": "ca.crt"
          }
        }
        ```

    - `caCertificateFile` - read the CA certificate from a file on disk. Useful
      when the provider Pod has a volume-mounted Secret or ConfigMap.

        ```json
        "tls": {
          "caCertificateFile": "/etc/certs/ca.crt"
        }
        ```

    **mTLS**: To additionally present a client certificate (mutual TLS), use one
    of two methods:

    - `clientCertificateSecretRef` - reference a Kubernetes Secret containing the
    certificate and key (default fields: `tls.crt` and `tls.key`, compatible with
    cert-manager). Override field names with `certField` and `keyField`.

      ```json
      "tls": {
        "clientCertificateSecretRef": {
          "name": "kafka-client-certs",
          "namespace": "kafka-cluster",
          "certField": "tls.crt",
          "keyField": "tls.key"
        }
      }
      ```

    - `clientCertificatePath` — read the certificate and key directly from files
      on disk. Useful when the provider Pod has a volume-mounted Secret or a
      cert-manager Certificate projected into the filesystem.

        ```json
        "tls": {
          "clientCertificatePath": {
            "certFile": "/etc/certs/tls.crt",
            "keyFile": "/etc/certs/tls.key"
          }
        }
        ```

    Both mTLS options may be configured, but they are not combined in the same
    TLS handshake. If `clientCertificatePath` is set, it takes precedence over
    `clientCertificateSecretRef` and its certificate/key will be used. Otherwise,
    the certificate/key from `clientCertificateSecretRef` will be used.

    **AWS MSK IAM**: When using `aws-msk-iam`, the provider uses the default AWS
    credential chain (environment variables, IRSA, etc.). The IAM role needs at
    minimum the following permissions to manage topics:

      ```json
      {
        "Action": [
          "kafka-cluster:Connect",
          "kafka-cluster:CreateTopic",
          "kafka-cluster:DeleteTopic",
          "kafka-cluster:DescribeTopic",
          "kafka-cluster:DescribeTopicDynamicConfiguration",
          "kafka-cluster:AlterTopic",
          "kafka-cluster:AlterTopicDynamicConfiguration"
        ],
        "Effect": "Allow",
        "Resource": [
          "arn:aws:kafka:<aws-region>:<aws-account-id>:cluster/<cluster-name>/<cluster-id>",
          "arn:aws:kafka:<aws-region>:<aws-account-id>:topic/<cluster-name>/<cluster-id>/<topic-name>"
        ]
      }
      ```

2. Create a k8s secret containing above config:

    ```console
    kubectl -n crossplane-system create secret generic kafka-creds --from-file=credentials=kc.json
    ```

3. Create a `ProviderConfig`, see [providerconfig examples](examples/namespaced/providerconfig/).

4. Create a managed resource, see [topic](examples/namespaced/topic/) and [acl](examples/namespaced/acl/) for examples.

### Importing existing resources

You can import existing resources into Crossplane by using the `Observe` management policy.

To import an existing topic, set the `crossplane.io/external-name` annotation to
the topic name as follows:

```yaml
apiVersion: topic.kafka.m.crossplane.io/v1alpha1
kind: Topic
metadata:
  name: imported-topic
  annotations:
    crossplane.io/external-name: cluster-sample-topic
spec:
  managementPolicies:
    - Observe
  forProvider:
    replicationFactor: 3
    partitions: 6
  providerConfigRef:
    name: default
    kind: ClusterProviderConfig
```

The provider will observe the topic and populate `status.atProvider` with the
actual state without making any changes to the Kafka cluster.

> **Note**: Importing ACLs via `Observe` is not supported. Kafka ACLs don't have
> a unique identifier — they are identified by the full combination of their
> fields (resource name, type, principal, host, operation, permission type, and
> pattern type), making observe-only imports impractical.

## Development

Usually the only command you may need to run is:

`make review`

For more detailed development instructions, continue reading below.

### Setting up a Development Kafka Cluster

The following instructions will setup a development environment where you will have a locally running Kafka
installation (SASL-Plain enabled). To change the configuration of your instance further, please see available helm
parameters [here](https://github.com/bitnami/charts/tree/master/bitnami/kafka/#installing-the-chart).

> steps 1-5 can be done with `make test`

1. (Optional) Create a local [kind](https://kind.sigs.k8s.io/) cluster unless you want to develop against an existing
   k8s cluster.

   > Or simply run: `make kind-setup` or `make unit-tests.init` for steps 1-2.

2. Run `make kind-kafka-setup` or manually as follows:

    Install the [Kafka helm chart](https://bitnami.com/stack/kafka/helm):

    ```shell
    helm repo add bitnami https://charts.bitnami.com/bitnami
    helm repo update bitnami
    helm upgrade --install kafka-dev -n kafka-cluster bitnami/kafka \
      --create-namespace \
      --version 32.4.3 \
      --set image.repository=bitnamilegacy/kafka \
      --set auth.clientProtocol=sasl \
      --set deleteTopicEnable=true \
      --set authorizerClassName="kafka.security.authorizer.AclAuthorizer" \
      --set controller.replicaCount=1 \
      --wait
    ```

    Username is `user1`, obtain password using the following:

    ```shell
    export KAFKA_PASSWORD=$(kubectl get secret kafka-dev-user-passwords -oyaml | yq '.data.client-passwords | @base64d')
    ```

    Create the Kubernetes secret to be used by the `ProviderConfig` with:

    ```shell
    cat <<EOF > /tmp/creds.json
    {
      "brokers": [
          "kafka-dev-controller-headless.kafka-cluster.svc:9092"
      ],
      "sasl": {
          "mechanism": "PLAIN",
          "username": "user1",
          "password": "${KAFKA_PASSWORD}"
      }
    }
    EOF

    kubectl -n kafka-cluster create secret generic kafka-creds \
      --from-file=credentials=/tmp/creds.json
    ```

3. Install [kubefwd](https://github.com/txn2/kubefwd#os).

4. Run `kubefwd` for `kafka-cluster` namespace which will make internal k8s services locally accessible:

    ```console
    sudo kubefwd svc -n kafka-cluster -c ~/.kube/config
    ```

5. To run tests, use the `KAFKA_PASSWORD` environment variable from step 2

6. (optional) Install the [kafka cli](https://github.com/twmb/kcl) and:

    1. Create a config file for the client with:

        ```shell
        cat <<EOF > ~/.kcl/config.toml
        seed_brokers = ["kafka-dev-controller-0.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092","kafka-dev-controller-1.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092","kafka-dev-controller-2.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092"]
        timeout_ms = 10000
        [sasl]
        method = "plain"
        user = "user1"
        pass = "${KAFKA_PASSWORD}"
        EOF
        ```

    2. Verify that cli could talk to the Kafka cluster:

        ```shell
        export  KCL_CONFIG_DIR=~/.kcl
        
        kcl metadata --all
        ```

7. (optional) or deploy [RedPanda console](https://github.com/redpanda-data/console) with:

    ```shell
    kubectl create -f - <<EOF
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: rp-console
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: rp-console
      template:
        metadata:
          labels:
          app: rp-console
        spec:
          containers:
            - name: rp-console
              image: docker.redpanda.com/redpandadata/console:latest
              ports:
                - containerPort: 8001
              env:
                - name: KAFKA_TLS_ENABLED
                  value: "false"
                - name: KAFKA_SASL_ENABLED
                  value: "true"
                - name: KAFKA_SASL_USERNAME
                  value: user1
                - name: KAFKA_SASL_PASSWORD
                  value: ${KAFKA_PASSWORD}
                - name: KAFKA_BROKERS
                  value: kafka-dev-controller-headless.kafka-cluster.svc:9092
    EOF
    ```

### Building and Running the provider locally

Run against a Kubernetes cluster:

```yaml
# Install CRD and run provider locally (out-of-cluster)
make dev

# Create a ProviderConfig pointing to the local Kafka cluster
kubectl apply -f - <<EOF
apiVersion: kafka.m.crossplane.io/v1alpha1
kind: ClusterProviderConfig
metadata:
  name: default
spec:
  credentials:
    secretRef:
      key: credentials
      name: kafka-creds
      namespace: kafka-cluster
    source: Secret
EOF
```

### Building and deploying the provider in-cluster

Build the provider image and deploy it as a Crossplane Provider package in the
kind cluster:

```console
make local-deploy
```

Build package:

```console
make build
```

Build image:

```console
make image
```

Push image:

```console
make push
```

Build binary:

```console
make build
```
