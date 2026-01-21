# ====================================================================================
# Setup Project

PROJECT_NAME := provider-kafka
PROJECT_REPO := github.com/crossplane-contrib/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

-include build/makelib/output.mk

# ====================================================================================
# Setup Go

NPROCS ?= 1
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
GOLANGCILINT_VERSION = 2.8.0
-include build/makelib/golang.mk

# ====================================================================================
# Setup Kubernetes tools

HELM_VERSION = v3.19.4
KIND_VERSION = v0.31.0
KUBECTL_VERSION = v1.35.0
UP_CHANNEL = stable
UP_VERSION = v0.37.0
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images

IMAGES = provider-kafka
-include build/makelib/imagelight.mk

# ====================================================================================
# Setup XPKG

XPKG_REG_ORGS ?= xpkg.upbound.io/crossplane-contrib
# NOTE(hasheddan): skip promoting on xpkg.upbound.io as channel tags are
# inferred.
XPKG_REG_ORGS_NO_PROMOTE ?= xpkg.upbound.io/crossplane-contrib
XPKGS = provider-kafka
-include build/makelib/xpkg.mk

# NOTE(hasheddan): we force image building to happen prior to xpkg build so that
# we ensure image is present in daemon.
xpkg.build.provider-kafka: do.build.images

fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# integration tests
e2e.run: test-integration

# Run integration tests.
test-integration: $(KIND) $(KUBECTL) $(CROSSPLANE_CLI) $(HELM3)
	@$(INFO) running integration tests using kind $(KIND_VERSION)
	@KIND_NODE_IMAGE_TAG=${KIND_NODE_IMAGE_TAG} $(ROOT_DIR)/cluster/local/integration_tests.sh || $(FAIL)
	@$(OK) integration tests passed

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# NOTE(hasheddan): the build submodule currently overrides XDG_CACHE_HOME in
# order to force the Helm 3 to use the .work/helm directory. This causes Go on
# Linux machines to use that directory as the build cache as well. We should
# adjust this behavior in the build submodule because it is also causing Linux
# users to duplicate their build cache, but for now we just make it easier to
# identify its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

go.mod.cachedir:
	@go env GOMODCACHE

# NOTE(hasheddan): we must ensure up is installed in tool cache prior to build
# as including the k8s_tools machinery prior to the xpkg machinery sets UP to
# point to tool cache.
build.init: $(CROSSPLANE_CLI)

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	@# To see other arguments that can be provided, run the command with --help instead
	$(GO_OUT_DIR)/provider --debug

.PHONY: submodules fallthrough test-integration run

# ====================================================================================
# Special Targets

# Install gomplate
GOMPLATE_VERSION := 3.10.0
GOMPLATE := $(TOOLS_HOST_DIR)/gomplate-$(GOMPLATE_VERSION)

$(GOMPLATE):
	@$(INFO) installing gomplate $(SAFEHOSTPLATFORM)
	@mkdir -p $(TOOLS_HOST_DIR)
	@curl -fsSLo $(GOMPLATE) https://github.com/hairyhenderson/gomplate/releases/download/v$(GOMPLATE_VERSION)/gomplate_$(SAFEHOSTPLATFORM) || $(FAIL)
	@chmod +x $(GOMPLATE)
	@$(OK) installing gomplate $(SAFEHOSTPLATFORM)

export GOMPLATE

# This target prepares repo for your provider by replacing all "template"
# occurrences with your provider name.
# This target can only be run once, if you want to rerun for some reason,
# consider stashing/resetting your git state.
# Arguments:
#   provider: Camel case name of your provider, e.g. GitHub, PlanetScale
provider.prepare:
	@[ "${provider}" ] || ( echo "argument \"provider\" is not set"; exit 1 )
	@PROVIDER=$(provider) ./hack/helpers/prepare.sh

# This target adds a new api type and its controller.
# You would still need to register new api in "apis/<provider>.go" and
# controller in "internal/controller/<provider>.go".
# Arguments:
#   provider: Camel case name of your provider, e.g. GitHub, PlanetScale
#   group: API group for the type you want to add.
#   kind: Kind of the type you want to add
#	apiversion: API version of the type you want to add. Optional and defaults to "v1alpha1"
provider.addtype: $(GOMPLATE)
	@[ "${provider}" ] || ( echo "argument \"provider\" is not set"; exit 1 )
	@[ "${group}" ] || ( echo "argument \"group\" is not set"; exit 1 )
	@[ "${kind}" ] || ( echo "argument \"kind\" is not set"; exit 1 )
	@PROVIDER=$(provider) GROUP=$(group) KIND=$(kind) APIVERSION=$(apiversion) PROJECT_REPO=$(PROJECT_REPO) ./hack/helpers/addtype.sh

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.

endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special

# ====================================================================================
# Development and Testing

dev: $(KIND) $(KUBECTL) $(DOCKER)
	@($(MAKE) -s kind-setup)
	@$(INFO) Starting Provider Kafka controllers
	@$(GO) run cmd/provider/main.go --debug

kind-setup: $(KIND)
	@$(KIND) get clusters | grep $(PROJECT_NAME)-dev || ( \
		$(INFO) Creating kind cluster; \
		$(KIND) create cluster --name=$(PROJECT_NAME)-dev --quiet --wait 5m; \
	)
	@$(KIND) export kubeconfig --name $(PROJECT_NAME)-dev
	@$(HELM) repo add crossplane-stable https://charts.crossplane.io/stable
	@$(HELM) repo update crossplane-stable
	@$(HELM) upgrade --install crossplane --create-namespace --namespace crossplane-system crossplane-stable/crossplane --wait
	@$(INFO) Installing Provider Kafka CRDs
	@$(KUBECTL) apply -R -f package/crds

kind-kafka-setup: $(HELM) $(KIND) $(KUBECTL)
	@$(INFO) Installing Kafka cluster in kind
	@$(HELM) repo add strimzi https://strimzi.io/charts
	@$(HELM) repo update strimzi
	@$(HELM) upgrade --install kafka-operator strimzi/strimzi-kafka-operator \
		--create-namespace --namespace kafka-operator \
		--version 0.50.0 \
		--set watchAnyNamespace=true \
		--wait
	@$(KUBECTL) create namespace kafka-cluster --dry-run=client -o yaml | $(KUBECTL) apply -f -
	@$(KUBECTL) apply -f cluster/local/kafka-cluster.yaml
	@$(INFO) Creating Kafka cluster and waiting for readiness...
	@$(KUBECTL) wait --for=condition=ready -n kafka-cluster kafka/dev --timeout=300s
	@$(KUBECTL) wait --for=condition=ready -n kafka-cluster kafkauser/user --timeout=300s
	@$(KUBECTL) wait --for=condition=ready -n kafka-cluster kafkatopic/pre-existing --timeout=300s
	@$(INFO) Getting service IP and port
	@KIND_NODE_IP=$$($(KIND) get nodes --name=$(PROJECT_NAME)-dev | \
	xargs $(KUBECTL) get node -o jsonpath='{.status.addresses[?(@.type=="InternalIP")].address}') && \
	KAFKA_NODEPORT=$$($(KUBECTL) -n kafka-cluster get svc dev-kafka-plain-bootstrap -o jsonpath='{.spec.ports[0].nodePort}') && \
	KAFKA_PASSWORD=$$($(KUBECTL) get secret user -n kafka-cluster -o jsonpath='{.data.password}' | base64 -d) && \
	echo "{ \
		\"brokers\": [ \
			\"$${KIND_NODE_IP}:$${KAFKA_NODEPORT}\" \
		], \
		\"sasl\": { \
			\"mechanism\": \"SCRAM-SHA-512\", \
			\"username\": \"user\", \
			\"password\": \"$${KAFKA_PASSWORD}\" \
		} \
	}" | tee kc.json
	@$(KUBECTL) -n kafka-cluster create secret generic kafka-creds --from-file=credentials=kc.json \
	--dry-run=client -o yaml | $(KUBECTL) apply -f -

sbom:
	@$(INFO) Generating SBOM
	@go install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@v1.9.0
	@cyclonedx-gomod mod -output provider-kafka-sbom.xml -output-version 1.6
	@$(OK) SBOM generated at provider-kafka-sbom.xml

review:
	@$(MAKE) reviewable
	@$(MAKE) sbom
	
test: unit-tests.init unit-tests.run unit-tests.done

unit-tests.init: $(HELM) $(KIND) $(KUBECTL)
	@$(MAKE) -s kind-setup
	@$(MAKE) -s kind-kafka-setup

unit-tests.done: $(KIND) $(KUBECTL)
	@$(INFO) Deleting kind cluster
	@$(KIND) delete cluster --name=$(PROJECT_NAME)-dev

unit-tests.run: $(HELM) $(KIND) $(KUBECTL)
	@KAFKA_CONFIG=$$($(KUBECTL) get secret kafka-creds -n kafka-cluster -o jsonpath='{.data.credentials}' | base64 -d) $(MAKE) -j2 -s go.test.unit

.PHONY: dev kind-setup kind-kafka-setup review sbom test