#!/bin/bash
set -x
# Usage
function usage {
  prereqs
	cat <<EOM
Usage: $(basename "$0") [OPTION]...
  -x          Install or reinstall crossplane
  -k          Install or reinstall kafka
  -c          Configure credentials
  -a          Run all three -xkc flags
  -h          Display help
EOM

	exit 2
}

crossplane(){
  echo "Installing crossplane.  Did you already install k8s and is it running?"
  kubectl delete namespace crossplane-system
  kubectl create namespace crossplane-system
  helm repo add crossplane-stable https://charts.crossplane.io/stable
  helm repo update
  helm install crossplane --namespace crossplane-system crossplane-stable/crossplane
}

prereqs(){
  echo "Did you already install helm, kubectl, and minikube?  Is minikube running?"
  echo "If not, hit up that README first!"
}
endout(){
  echo "Next steps: run 'make dev' from your command line to generate your CRDS and apply them."
  echo "This will also run your provider and allow you to apply a topic."
  echo "Then run 'sudo -E kubefwd svc -n kafka-cluster' to appropriately forward your K8s traffic around."
}

# Refresh the k8s kafka cluster
kafka(){
  prereqs
  echo "--- Deleting the kafka-cluster namespace"
  kubectl delete ns kafka-cluster;
  echo "--- Adding bitnami repo"
  helm repo add bitnami https://charts.bitnami.com/bitnami;
  echo "--- Creating the kafka-cluster namespace"
  kubectl create ns kafka-cluster;
  echo "--- Installing kafka-dev via helm"
  helm upgrade --install kafka-dev -n kafka-cluster bitnami/kafka \
    --version 15.0.1 \
    --set auth.clientProtocol=sasl \
    --set deleteTopicEnable=true \
    --set authorizerClassName="kafka.security.authorizer.AclAuthorizer" \
    --wait
}

cfgkcl(){
  echo "--- Setting up kcl credentials"
  mkdir ~/.kcl;
  touch ~/.kcl/config.toml;
  sed 's/<your-password>/'"$CREDS"'/g' devcliconfig.toml > ~/.kcl/config.toml;
  export  KCL_CONFIG_DIR=~/.kcl
}

cfgkaf(){
  echo "--- Setting up kaf credentials"
  sed 's/<your-password>/'"$CREDS"'/g' devkafcliconfig > ~/.kaf/config;
  endout
}

# Configure the credentials for kaf cli and/or kcl cli
config(){
  echo "This option is only for when kafka is already successfully installed."
  echo "--- Setting up cluster credentials"
  export CREDS=$(kubectl -n kafka-cluster exec kafka-dev-0 -- cat /opt/bitnami/kafka/config/kafka_jaas.conf | grep password | awk -F\" '{print $2}');
  sed 's/<your-password>/'"$CREDS"'/g' devcreds.json > kc.json;
  echo "--- Cleaning up old cluster credentials"
  kubectl -n crossplane-system delete secret kafka-creds;
  kubectl -n default delete secret kafka-creds;
  kubectl -n kafka-cluster delete secret kafka-creds;
  kubectl -n crossplane-system create secret generic kafka-creds --from-file=credentials=kc.json;
  cfgkcl
  cfgkaf
}

# Set up the options
while getopts ":xkcha" optKey; do
	case "$optKey" in
    x)
      crossplane
      ;;
		k)
			kafka
			;;
		c)
			config
			;;
    a)
      crossplane
      kafka
      config
      ;;
		h|*)
			usage
			;;
	esac
done
if [ $OPTIND -eq 1 ]; then usage; fi
shift $((OPTIND - 1))