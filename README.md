# provider-kafka

`provider-kafka` is a [Crossplane](https://crossplane.io/) Provider that is used to
manage [Kafka](https://kafka.apache.org/) resources.

## Usage

1. Create a provider secret containing a json like the following, see expected
   schema [here](internal/clients/kafka/config.go):

    ```
    {
      "brokers":[
        "kafka-dev-0.kafka-dev-headless:9092"
       ],
       "sasl":{
         "mechanism":"PLAIN",
         "username":"user",
         "password":"<your-password>"
       }
    }
    ```

2. Create a k8s secret containing above config:

    ```
    kubectl -n crossplane-system create secret generic kafka-creds --from-file=credentials=kc.json
    ```

3. Create a `ProviderConfig`, see [this](examples/provider/config.yaml) as an example.


4. Create a managed resource see, see [this](examples/topic/topic.yaml) for an example creating a `Kafka topic`.

## Development

### Setting up a Development Kafka Cluster

The following instructions will setup a development environment where you will have a locally running Kafka
installation (SASL-Plain enabled). To change the configuration of your instance further, please see available helm
parameters [here](https://github.com/bitnami/charts/tree/master/bitnami/kafka/#installing-the-chart).

1. (Optional) Create a local [kind](https://kind.sigs.k8s.io/) cluster unless you want to develop against an existing
   k8s cluster.


2. Install the [Kafka helm chart](https://bitnami.com/stack/kafka/helm):

      ```
      helm repo add bitnami https://charts.bitnami.com/bitnami
      kubectl create ns kafka-cluster
      helm upgrade --install kafka-dev -n kafka-cluster bitnami/kafka --set auth.clientProtocol=sasl --set deleteTopicEnable=true --wait
      ```

   Username is "user", obtain password using the following

      ```
      kubectl -n kafka-cluster exec kafka-dev-0 -- cat /opt/bitnami/kafka/config/kafka_jaas.conf
      ```
   Create the Kubernetes secret by adding a JSON filed called kc.json with the following contents
   ```json
   {
      "brokers": [
         "kafka-dev-0.kafka-dev-headless:9092"
      ],
      "sasl": {
         "mechanism": "PLAIN",
         "username": "user",
         "password": "<password-you-obtained-in-step-2>"
      }
   }
   ```
   Once this file is created, apply it by running the following command
    ```bash
    kubectl -n default create secret generic kafka-creds --from-file=credentials=kc.json
    ```

3. Install [kubefwd](https://github.com/txn2/kubefwd#os).


4. Run `kubefwd` for `kafka-cluster` namespace which will make internal k8s services locally accessible:

      ```
      sudo kubefwd svc -n kafka-cluster
      ```

5. (optional) Install [kafka cli](https://github.com/birdayz/kaf).


6. (optional) Configure the kafka cli to talk against local Kafka installation:

    1. Create a config file for the client with the following content at `~/.kaf/config`:

       ```
       current-cluster: local
       clusteroverride: ""
       clusters:
       - name: local
         version: ""
         brokers:
         - kafka-dev-0.kafka-dev-headless:9092
         SASL:
           mechanism: PLAIN
           username: user
           password: <password-you-obtained-in-step-2>
         TLS: null
         security-protocol: ""
         schema-registry-url: ""
       ```

        1. Verify that cli could talk to the Kafka cluster:

       ```
       kaf nodes
       ```

### Building and Running the provider locally

Run against a Kubernetes cluster:

```console
make run
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


