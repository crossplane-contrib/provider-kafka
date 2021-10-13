# provider-kafka

`provider-kafka` is a [Crossplane](https://crossplane.io/) Provider
that is used to manage [Kafka](https://kafka.apache.org/) resources.

## Developing

### Setting up a Development Kafka Cluster

The following instructions will setup a development environment where you will
have a locally running Kafka installation (SASL-Plain enabled). To change the
configuration of your instance further, please see available helm parameters [here](https://github.com/bitnami/charts/tree/master/bitnami/kafka/#installing-the-chart).

1. (Optional) Create a kind cluster unless you want to develop against an 
  existing k8s cluster. 

2. Install Kafka:

  https://bitnami.com/stack/kafka/helm
  
  ```
  helm repo add bitnami https://charts.bitnami.com/bitnami
  kubectl create ns kafka-cluster
  helm upgrade --install kafka-dev -n kafka-cluster bitnami/kafka --set auth.clientProtocol=sasl --set deleteTopicEnable=true --wait
  ```
  
  Username is "user", obtain password using the following
  
  ```
  kubectl -n kafka-cluster exec kafka-dev-0 -- cat /opt/bitnami/kafka/config/kafka_jaas.conf
  ```

3. Install [kubefwd](https://github.com/txn2/kubefwd#os)


4. Run `kubefwd` for `kafka-cluster` namespace which will make internal k8s 
services locally accessible.

  ```
  sudo kubefwd svc -n kafka-cluster
  ```

5. (optional) Install [kafka cli](https://github.com/birdayz/kaf)

6. (optional) Configure the kafka cli to talk against local Kafka installation:

   1. Create a config file for the client with the following content at `~/.kaf/config`

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
   
   1. Verify that cli could talk to the Kafka cluster
   
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
