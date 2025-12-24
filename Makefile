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
GOLANGCILINT_VERSION = 2.7.2
-include build/makelib/golang.mk

# ====================================================================================
# Setup Kubernetes tools

HELM_VERSION = v3.19.4
KIND_VERSION = v0.31.0
KUBECTL_VERSION = v1.35.0
KUBEFWD_VERSION = v1.23.2
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

dev: $(KIND) $(KUBECTL) $(DOCKER)
	@($(MAKE) -s kind-setup)
	@$(INFO) Starting Provider Kafka controllers
	@$(GO) run cmd/provider/main.go --debug

dev-clean: $(KIND) $(KUBECTL)
	@$(INFO) Deleting kind cluster
	@$(KIND) delete cluster --name=$(PROJECT_NAME)-dev

kind-setup: $(KIND)
	@$(KIND) get clusters | grep $(PROJECT_NAME)-dev || ( \
		$(INFO) Creating kind cluster; \
		$(KIND) create cluster --name=$(PROJECT_NAME)-dev --quiet; \
	)
	@$(KIND) export kubeconfig --name $(PROJECT_NAME)-dev
	@$(HELM) repo add crossplane-stable https://charts.crossplane.io/stable
	@$(HELM) repo update crossplane-stable
	@$(HELM) upgrade --install crossplane --create-namespace --namespace crossplane-system crossplane-stable/crossplane --wait
	@$(INFO) Installing Provider Kafka CRDs
	@$(KUBECTL) apply -R -f package/crds

kind-kafka-setup: $(HELM) $(KIND) $(KUBECTL)
	@$(INFO) Installing Kafka cluster in kind
	@$(HELM) repo add bitnami https://charts.bitnami.com/bitnami
	@$(HELM) repo update bitnami
	@$(HELM) upgrade --install kafka-dev bitnami/kafka \
		--create-namespace --namespace kafka-cluster \
		--version 32.4.3 \
		--set image.repository=bitnamilegacy/kafka \
		--set auth.clientProtocol=sasl \
		--set deleteTopicEnable=true \
		--set authorizerClassName="kafka.security.authorizer.AclAuthorizer" \
		--wait
	@KAFKA_PASSWORD=$($(KUBECTL) get secret kafka-dev-user-passwords -n kafka-cluster -o jsonpath='{.data.client-passwords}' | base64 -d | cut -d , -f 1)
	@echo "{ \
		\"brokers\": [ \
			\"kafka-dev-controller-0.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092\", \
			\"kafka-dev-controller-1.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092\", \
			\"kafka-dev-controller-2.kafka-dev-controller-headless.kafka-cluster.svc.cluster.local:9092\" \
		], \
		\"sasl\": { \
			\"mechanism\": \"PLAIN\", \
			\"username\": \"user\", \
			\"password\": \"${KAFKA_PASSWORD}\" \
		} \
	}" | tee kc.json
	@$(KUBECTL) -n kafka-cluster get secret kafka-creds > /dev/null && $(KUBECTL) -n kafka-cluster delete secret kafka-creds > /dev/null || true
	@$(KUBECTL) -n kafka-cluster create secret generic kafka-creds --from-file=credentials=kc.json

kind-cluster: $(HELM) $(KIND) $(KUBECTL)
	@$(MAKE) -s kind-setup
	@$(MAKE) -s kind-kafka-setup

unit-tests: $(HELM) $(KIND) $(KUBECTL)
# TODO: replace with another kafka helm chart
	@test -f $(TOOLS_HOST_DIR)/kubefwd || curl -fsSL "https://github.com/txn2/kubefwd/releases/download/${KUBEFWD_VERSION}/kubefwd_Linux_x86_64.tar.gz" -o - | tar zxvf - -C $(TOOLS_HOST_DIR) kubefwd
	@sudo killall kubefwd > /dev/null || true
	@sudo -E $(TOOLS_HOST_DIR)/kubefwd svc kafka-dev -n kafka-cluster -c ~/.kube/config &
	@KAFKA_PASSWORD=$($(KUBECTL) get secret kafka-dev-user-passwords -n kafka-cluster -o jsonpath='{.data.client-passwords}' | base64 -d | cut -d , -f 1); \
	export KAFKA_PASSWORD=$$KAFKA_PASSWORD; $(MAKE) -j2 -s test
	@sudo killall kubefwd
	@$(MAKE) -s dev-clean

.PHONY: submodules fallthrough test-integration run dev dev-clean

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