# ====================================================================================
# Versions

include versions.mk

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
GO_REQUIRED_VERSION ?= $(shell grep '^go ' go.mod | awk '{print $$2}')
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_SUBDIRS += cmd internal apis
GO111MODULE = on
export GOTOOLCHAIN := go$(GO_REQUIRED_VERSION)
-include build/makelib/golang.mk

# ====================================================================================
# Setup Kubernetes tools

UP_CHANNEL = stable
UP := $(TOOLS_HOST_DIR)/up-$(UP_VERSION)
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images

REGISTRY_ORGS ?= ghcr.io/crossplane-contrib
IMAGES = provider-kafka
-include build/makelib/imagelight.mk

# ====================================================================================
# Setup XPKG

KIND_CLUSTER_NAME = $(PROJECT_NAME)-dev

XPKG_REG_ORGS ?= xpkg.upbound.io/crossplane-contrib
# NOTE(hasheddan): skip promoting on xpkg.upbound.io as channel tags are
# inferred.
XPKG_REG_ORGS_NO_PROMOTE ?= xpkg.upbound.io/crossplane-contrib
XPKGS = $(PROJECT_NAME)
-include build/makelib/xpkg.mk
-include build/makelib/local.xpkg.mk

# NOTE(hasheddan): we force image building to happen prior to xpkg build so that
# we ensure image is present in daemon.
xpkg.build.provider-kafka: do.build.images

# ====================================================================================
# Package Extensions (readme, SBOM)
# See: https://docs.upbound.io/manuals/marketplace/packages/#add-documentation-icons-and-other-assets-to-your-package

EXTENSIONS_DIR := $(ROOT_DIR)/extensions

$(UP):
	@$(INFO) installing up $(UP_VERSION)
	@mkdir -p $(TOOLS_HOST_DIR)
	@curl -fsSLo $(UP) https://cli.upbound.io/$(UP_CHANNEL)/$(UP_VERSION)/bin/$(SAFEHOST_PLATFORM)/up || $(FAIL)
	@chmod +x $(UP)
	@$(OK) installing up $(UP_VERSION)

xpkg.extensions: sbom
	@$(INFO) Preparing package extensions
	@mkdir -p $(EXTENSIONS_DIR)/readme
	@cp $(ROOT_DIR)/README.md $(EXTENSIONS_DIR)/readme/readme.md
	@$(OK) Package extensions prepared at $(EXTENSIONS_DIR)

xpkg.append: xpkg.extensions $(UP)
	@$(INFO) Appending extensions to $(XPKG_REG_ORGS)/$(PROJECT_NAME):$(VERSION)
	@$(UP) alpha xpkg append --extensions-root=$(EXTENSIONS_DIR) $(XPKG_REG_ORGS)/$(PROJECT_NAME):$(VERSION) || $(FAIL)
	@$(OK) Appended extensions to $(XPKG_REG_ORGS)/$(PROJECT_NAME):$(VERSION)

go.cachedir:
	@go env GOCACHE

go.mod.cachedir:
	@go env GOMODCACHE

fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

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

.PHONY: submodules fallthrough run

# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.
    dev                   Create kind cluster with Crossplane and run provider locally.
    deploy                Build and deploy provider as a Crossplane package in kind.

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

-include build/makelib/controlplane.mk

local-deploy: build.all controlplane.up local.xpkg.deploy.provider.$(PROJECT_NAME)
	@$(INFO) running locally built provider
	@$(KUBECTL) wait provider.pkg $(PROJECT_NAME) --for condition=Healthy --timeout 5m
	@$(KUBECTL) -n crossplane-system wait --for=condition=Available deployment --all --timeout=5m
	@$(OK) running locally built provider

dev: $(KIND) $(KUBECTL) $(DOCKER)
	@($(MAKE) -s kind-setup)
	@$(INFO) Starting Provider Kafka controllers
	@$(GO) run cmd/provider/main.go --debug

kind-setup: $(KIND) $(HELM) $(KUBECTL)
	@$(KIND) get clusters | grep $(KIND_CLUSTER_NAME) || ( \
		$(INFO) Creating kind cluster; \
		$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --quiet --wait 5m; \
	)
	@$(KIND) export kubeconfig --name $(KIND_CLUSTER_NAME)
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
		--version $(STRIMZI_CHART_VERSION) \
		--set watchAnyNamespace=true \
		--wait
	@$(KUBECTL) create namespace kafka-cluster --dry-run=client -o yaml | $(KUBECTL) apply -f -
	@$(KUBECTL) apply -f cluster/local/kafka-cluster.yaml
	@$(INFO) Creating Kafka cluster and waiting for readiness...
	@$(KUBECTL) wait --for=condition=ready -n kafka-cluster kafka/dev --timeout=300s
	@$(KUBECTL) wait --for=condition=ready -n kafka-cluster kafkauser/user --timeout=300s
	@$(KUBECTL) wait --for=condition=ready -n kafka-cluster kafkatopic/pre-existing --timeout=300s
	@$(KUBECTL) annotate kafkatopic --all -n kafka-cluster strimzi.io/pause-reconciliation=true --overwrite
	@$(INFO) Waiting for Kafka broker to be fully initialized...
	@for i in $$(seq 1 30); do $(KUBECTL) -n kafka-cluster exec kafka-dev-0 -- bash -c 'echo "test" | kafka-console-producer.sh --broker-list localhost:9092 --topic pre-existing' > /dev/null 2>&1 && break || sleep 2; done
	@$(INFO) Getting service IP and port
	@KIND_NODE_IP=$$($(KIND) get nodes --name=$(KIND_CLUSTER_NAME) | \
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

review:
	@$(MAKE) reviewable
	@$(MAKE) sbom

SYFT := $(TOOLS_HOST_DIR)/syft-$(SYFT_VERSION)

$(SYFT):
	@$(INFO) installing syft $(SYFT_VERSION)
	@mkdir -p $(TOOLS_HOST_DIR)
	@curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b $(TOOLS_HOST_DIR) $(SYFT_VERSION) || $(FAIL)
	@mv $(TOOLS_HOST_DIR)/syft $(SYFT)
	@$(OK) installing syft $(SYFT_VERSION)

sbom: $(SYFT)
	@$(INFO) Generating SPDX SBOM
	@mkdir -p $(EXTENSIONS_DIR)/sbom
	@$(SYFT) scan dir:. --source-name $(PROJECT_NAME) --source-version $(VERSION) -o spdx-json=$(EXTENSIONS_DIR)/sbom/sbom.spdx.json
	@$(OK) SBOM generated at $(EXTENSIONS_DIR)/sbom/sbom.spdx.json
	
test: integration-tests.init integration-tests.run integration-tests.done

integration-tests.init: $(HELM) $(KIND) $(KUBECTL)
	@$(MAKE) -s kind-setup
	@$(MAKE) -s kind-kafka-setup

integration-tests.run: $(HELM) $(KIND) $(KUBECTL)
	@KAFKA_CONFIG=$$($(KUBECTL) get secret kafka-creds -n kafka-cluster -o jsonpath='{.data.credentials}' | base64 -d) $(MAKE) -j2 -s go.test.unit

integration-tests.done: $(KIND) $(KUBECTL)
	@$(INFO) Deleting kind cluster
	@$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME)

.PHONY: dev kind-setup kind-kafka-setup review sbom test xpkg.extensions xpkg.append
