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

2. Create a k8s secret containing above config:

    ```console
    kubectl -n crossplane-system create secret generic kafka-creds --from-file=credentials=kc.json
    ```

3. Create a `ProviderConfig`, see [this](examples/provider/config.yaml) as an example.


4. Create a managed resource see, see [this](examples/topic/topic.yaml) for an example creating a `Kafka topic`.

## Development

Usually the only command you may need to run is:

`make reviewable`

For more detailed development instructions, continue reading below.

### Setting up a Development Kafka Cluster

The following instructions will setup a development environment where you will have a locally running Kafka
installation (SASL-Plain enabled). To change the configuration of your instance further, please see available helm
parameters [here](https://github.com/bitnami/charts/tree/master/bitnami/kafka/#installing-the-chart).

> steps 1-5 can be done with `make test`

1. (Optional) Create a local [kind](https://kind.sigs.k8s.io/) cluster unless you want to develop against an existing
   k8s cluster.

   > Or simply run: `make kind-setup` or `make unit-tests.init` for steps 1-2.

2. Run `make kind-kafka-setup` or manually with:

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

6. (optional) Install the [kafka cli](https://github.com/twmb/kcl)
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
      
       1. Verify that cli could talk to the Kafka cluster:
      
         ```shell
         export  KCL_CONFIG_DIR=~/.kcl
         
         kcl metadata --all
         ```

6. (optional) or deploy [RedPanda console](https://github.com/redpanda-data/console) with:

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

```console
# Install CRD and run provider locally
make dev

# Create a ProviderConfig pointing to the local Kafka cluster
kubectl apply -f - <<EOF
kind: ProviderConfig
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

Build, push, and install:

```console
make all
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
