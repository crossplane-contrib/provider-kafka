# Authentication

## Usage

Currently, the only supported authentication mechanism is SASL.

### SASL_PLAIN

In order to use ```SASL_PLAIN```, create a provider secret containing a json like the following, see expected
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

### AWS_MSK_IAM

In order to use ```AWS_MSK_IAM```, create a provider secret containing a json like the following:

    ```
    {
      "brokers":[
        "kafka-dev-0.kafka-dev-headless:9092"
       ],
       "sasl":{
         "mechanism":"AWS_MSK_IAM",
         "roleArn": "arn:aws:iam::account:role/role-name-with-path" (optional)
       }
    }
    ```

The provider assumes that you have either set the appropriate environment variables for the provider or running on EKS with injected identity annotation, like following:
```
apiVersion: kafka.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: example
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::account:role/role-name-with-path
spec:
  ...
```