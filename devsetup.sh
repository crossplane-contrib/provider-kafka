set -x
echo "This script only installs the kafka development cluster via helm, and sets up the credentials.  You will need to follow the rest of the instructions to install helm, your kube cluster, etc.)"
kubectl delete ns kafka-cluster;
helm repo add bitnami https://charts.bitnami.com/bitnami;
kubectl create ns kafka-cluster;
helm upgrade --install kafka-dev -n kafka-cluster bitnami/kafka --set auth.clientProtocol=sasl --set deleteTopicEnable=true --set autoCreateTopicsEnable=false --wait --debug;
CREDS=$(kubectl -n kafka-cluster exec kafka-dev-0 -- cat /opt/bitnami/kafka/config/kafka_jaas.conf | grep password | awk -F\" '{print $2}');
sed 's/<your-password>/'"$CREDS"'/g' devcreds.json > kc.json;
sed 's/<your-password>/'"$CREDS"'/g' devkafcliconfig > ~/.kaf/config;
kubectl -n crossplane-system delete secret kafka-creds;
kubectl -n crossplane-system create secret generic kafka-creds --from-file=credentials=kc.json;
kaf config use-cluster local