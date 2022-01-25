set -x
echo "PREREQUISITES TO INSTALL: helm, kubectl, a running kube cluster, kcl cli, crossplane, kubefwd"
echo "See README for installation links and instructions."
kubectl delete ns kafka-cluster;
helm repo add bitnami https://charts.bitnami.com/bitnami;
kubectl create ns kafka-cluster;
helm upgrade --install kafka-dev -n kafka-cluster bitnami/kafka --set auth.clientProtocol=sasl --set deleteTopicEnable=true --set autoCreateTopicsEnable=false --set authorizerClassName="kafka.security.auth.SimpleAclAuthorizer" --wait --debug;
CREDS=$(kubectl -n kafka-cluster exec kafka-dev-0 -- cat /opt/bitnami/kafka/config/kafka_jaas.conf | grep password | awk -F\" '{print $2}');
sed 's/<your-password>/'"$CREDS"'/g' devcreds.json > kc.json;
mkdir ~/.kcl;
touch ~/.kcl/config.toml;
sed 's/<your-password>/'"$CREDS"'/g' devcliconfig.toml > ~/.kcl/config.toml;
kubectl -n crossplane-system delete secret kafka-creds;
kubectl -n crossplane-system create secret generic kafka-creds --from-file=credentials=kc.json;
export  KCL_CONFIG_DIR=~/.kcl
echo "Next steps: run 'make dev' from your command line to generate your CRDS and apply them."
echo "This will also run your provider and allow you to apply a topic."
echo "Then run 'sudo -E kubefwd svc -n kafka-cluster' to appropriately forward your K8s traffic around."