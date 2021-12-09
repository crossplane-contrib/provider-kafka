# ====================================================================================
# Setup Project

include ../../../build/makelib/common.mk

# ====================================================================================
#  Options

include ../../../build/makelib/imagelight.mk

# ====================================================================================
# Targets

img.build:
	@$(INFO) docker build $(IMAGE)
	@$(MAKE) BUILD_ARGS="--load" img.build.shared
	@$(OK) docker build $(IMAGE)

img.publish:
	@$(INFO) docker publish $(IMAGE)
	@$(MAKE) BUILD_ARGS="--push" img.build.shared
	@$(OK) docker publish $(IMAGE)

img.build.shared:
	@cp Dockerfile $(IMAGE_TEMP_DIR) || $(FAIL)
	@cp -R ../../../package . $(IMAGE_TEMP_DIR) || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|BASEIMAGE|$(OSBASEIMAGE)|g' Dockerfile || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|VERSION|$(VERSION)|g' package/crossplane.yaml || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && $(SED_CMD) 's|DOCKER_REGISTRY|$(DOCKER_REGISTRY)|g' package/crossplane.yaml || $(FAIL)
	@cd $(IMAGE_TEMP_DIR) && find package -type f -name '*.yaml' -exec cat {} >> 'package.yaml' \; -exec printf '\n---\n' \; || $(FAIL)	
	@docker buildx build $(BUILD_ARGS) \
		--platform $(IMAGE_PLATFORMS) \
		-t $(IMAGE) \
		$(IMAGE_TEMP_DIR) || $(FAIL)

img.promote:
	@$(INFO) docker promote $(FROM_IMAGE) to $(TO_IMAGE)
	@docker buildx imagetools create -t $(TO_IMAGE) $(FROM_IMAGE)
	@$(OK) docker promote $(FROM_IMAGE) to $(TO_IMAGE)