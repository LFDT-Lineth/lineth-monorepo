PROJECT_NAME := "linea-tracer"
VERSION := $(shell cat gradle.properties | grep releaseVersion | cut -d "=" -f 2)

SHADOW_NODE_ROOT := "/home/ec2-user/shadow-node"

DIST_NAME := "$(PROJECT_NAME)-$(VERSION)"
DIST_JAR_PATH := "arithmetization/build/libs/$(DIST_NAME).jar"
BESU_PLUGINS_PATH := "$(SHADOW_NODE_ROOT)/linea-besu/plugins"
ZKEVM_BIN_VERSIONED_NAME := $(shell echo zkevm.bin-$(shell git -C linea-constraints rev-parse --short HEAD))
ZKEVM_BIN_VERSIONED_PATH := "linea-constraints/$(ZKEVM_BIN_VERSIONED_NAME)"
ZKEVM_BIN_ORIGINAL_PATH := "linea-constraints/zkevm.bin"

ACCOUNT_FRAGMENT_FILE_PATH := "arithmetization/src/main/java/net/consensys/linea/zktracer/module/hub/fragment/account/AccountFragment.java"

# call this target with -> make node_address=<node_address> shadow-node-deploy
shadow-node-deploy:
	@sed -i -e 's/this\.existsInfinity = /\/\/ this\.existsInfinity = /g' $(ACCOUNT_FRAGMENT_FILE_PATH)
	@echo ">>>>>>>>>>> Building $(DIST_NAME) plugin jar file..."
	./gradlew --no-daemon clean jar
	@echo ">>>>>>>>>>> Copying $(DIST_NAME).jar to $(BESU_PLUGINS_PATH) on shadow node server..."
	@scp -r "$(DIST_JAR_PATH)" "$(node_address):$(BESU_PLUGINS_PATH)"
	@sed -i -e 's/\/\/ this\.existsInfinity = /this\.existsInfinity = /g' $(ACCOUNT_FRAGMENT_FILE_PATH)
	@echo ">>>>>>>>>>> Building zkevm.bin..."
	@make -C linea-constraints zkevm.bin -B && cp "$(ZKEVM_BIN_ORIGINAL_PATH)" "$(ZKEVM_BIN_VERSIONED_PATH)"
	@scp -r "$(ZKEVM_BIN_VERSIONED_PATH)" "$(node_address):$(SHADOW_NODE_ROOT)"
	@ssh -t "$(node_address)" \
		'cd $(SHADOW_NODE_ROOT); ln -sf $(ZKEVM_BIN_VERSIONED_NAME) zkevm.bin; find $(BESU_PLUGINS_PATH) ! -name $(DIST_NAME).jar -type f -exec rm -f {} +; zsh -l'
