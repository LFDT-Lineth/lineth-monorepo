include versions.env

BESU_PACKAGE_TAG ?= local
PROVER_TAG ?= local
PLATFORM ?=
BESU_COMMIT ?= $(shell grep -E '^\s*besuCommit\s*=' ../gradle/libs.versions.toml | sed -n 's/.*"\([^"]*\)".*/\1/p')
TRACER_PLUGIN_VERSION ?= linea-$(shell git rev-parse --short=7 HEAD)
SEQUENCER_PLUGIN_VERSION ?= linea-$(shell git rev-parse --short=7 HEAD)
LOCAL_SEQUENCER_DIST_FOLDER ?= $(abspath ../besu-plugins/linea-sequencer/sequencer/build/install/sequencer)
LOCAL_TRACER_DIST_FOLDER ?= $(abspath ../tracer/plugins/build/install/linea-tracer)
LOCAL_BESU_DIST_FOLDER ?= $(abspath ../tmp/besu-eth/build/distributions)

build-besu-local-and-update-lib-version:
	cd ..; \
	echo "Build besu-eth/besu locally at BESU_COMMIT=$(BESU_COMMIT)" ; \
	./gradlew buildAndUpdateBesuVersionInLibsVersions

build-tracer-and-sequencer:
	cd ..; \
	echo "Build versions: TRACER_PLUGIN_VERSION=${TRACER_PLUGIN_VERSION} SEQUENCER_PLUGIN_VERSION=$(SEQUENCER_PLUGIN_VERSION)" ; \
	echo "Clean and build distribution for tracer" ; \
	./gradlew :tracer:clean && ./gradlew :tracer:plugins:installDist -PreleaseVersion=$(TRACER_PLUGIN_VERSION) ; \
	echo "Clean and build distribution for sequencer" ; \
	./gradlew besu-plugins:linea-sequencer:sequencer:clean && ./gradlew besu-plugins:linea-sequencer:sequencer:installDist -PreleaseVersion=$(SEQUENCER_PLUGIN_VERSION)

assemble: BESU_VERSION := $(shell grep -E '^\s*besu\s*=' ../gradle/libs.versions.toml | sed -n 's/.*"\([^"]*\)".*/\1/p')
assemble: LOCAL_BESU_ZIP_PATH := $(LOCAL_BESU_DIST_FOLDER)/besu-$(BESU_VERSION).tar.gz
assemble:
	BESU_VERSION=$(BESU_VERSION) \
	LOCAL_BESU_ZIP_PATH=$(LOCAL_BESU_ZIP_PATH) \
	LOCAL_SEQUENCER_DIST_FOLDER=$(LOCAL_SEQUENCER_DIST_FOLDER) \
	LOCAL_TRACER_DIST_FOLDER=$(LOCAL_TRACER_DIST_FOLDER) \
	./scripts/assemble-packages.sh

build-image:
	echo "IMAGE TAG=$(BESU_PACKAGE_TAG)"
	echo "PLATFORM=$(PLATFORM)"
	if [ "$(PLATFORM)" = "" ]; then \
		echo "Building image for default platform"; \
		docker buildx build --file linea-besu/Dockerfile --tag consensys/linea-besu-package:$(BESU_PACKAGE_TAG) ./tmp --progress=plain --no-cache --load; \
	else \
		echo "Building image for $(PLATFORM)"; \
		docker buildx build --file linea-besu/Dockerfile --platform $(PLATFORM) --tag consensys/linea-besu-package:$(BESU_PACKAGE_TAG) ./tmp --progress=plain --no-cache --load; \
	fi

build:
	echo "Build linea-besu-package image from source with tag=$(BESU_PACKAGE_TAG)"
	$(MAKE) build-besu-local-and-update-lib-version
	$(MAKE) build-tracer-and-sequencer
	$(MAKE) clean
	$(MAKE) assemble
	$(MAKE) build-image

build-prover:
	echo "Build prover image from source with tag=$(PROVER_TAG)"
	cd ../prover; \
	echo "Build zkevm.bin for prover" ; \
	make zkevm/arithmetization/zkevm.bin ; \
	echo "Build prover docker image" ; \
	VERSION=$(PROVER_TAG) make docker

build-all:
	$(MAKE) build
	$(MAKE) build-prover

run-e2e-test-with-partial-prover:
	cd .. && LINEA_BESU_PACKAGE_TAG=$(BESU_PACKAGE_TAG) LINEA_PROVER_TAG=$(PROVER_TAG) make start-env-with-tracing-v2-partialprover && pnpm run -F e2e test:partial-prover:local

run-e2e-test:
	cd .. && LINEA_BESU_PACKAGE_TAG=$(BESU_PACKAGE_TAG) make start-env-with-tracing-v2-ci && pnpm run -F e2e test:local

run-e2e-test-cleanup:
	cd .. && make clean-environment

clean:
	rm -fr tmp || true
