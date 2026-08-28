include versions.env

BESU_PACKAGE_TAG ?= local
PROVER_TAG ?= local
PLATFORM ?=
BESU_COMMIT ?= $(shell grep -E '^\s*besuCommit\s*=' ../../gradle/libs.versions.toml | sed -n 's/.*"\([^"]*\)".*/\1/p')
TRACER_PLUGIN_VERSION ?= linea-$(shell git rev-parse --short=7 HEAD)
SEQUENCER_PLUGIN_VERSION ?= linea-$(shell git rev-parse --short=7 HEAD)
LOCAL_SEQUENCER_DIST_FOLDER ?= $(abspath ../plugins/linea-sequencer/sequencer/build/install/sequencer)
LOCAL_TRACER_DIST_FOLDER ?= $(abspath ../../tracer/plugins/build/install/linea-tracer)
LOCAL_BESU_DIST_FOLDER ?= $(abspath ../../tmp/besu-eth/build/distributions)
BESU_VERSION ?= $(shell ls ../../tmp/besu-eth/build/distributions/besu-*.tar.gz 2>/dev/null | head -1 | sed 's/.*besu-\(.*\)\.tar\.gz/\1/')
LOCAL_BESU_ZIP_PATH ?= $(LOCAL_BESU_DIST_FOLDER)/besu-$(BESU_VERSION).tar.gz

clean-besu-and-maven-local:
	cd ../..; \
	echo "Clean up tmp besu folder and maven local" ; \
	./gradlew :linea-besu:besu:clean

build-besu:
	cd ../..; \
	echo "Build besu-eth/besu locally at BESU_COMMIT=$(BESU_COMMIT)" ; \
	./gradlew :linea-besu:besu:build -PskipDownloadBesuDist=false

build-tracer-and-sequencer:
	cd ../..; \
	echo "Build versions: TRACER_PLUGIN_VERSION=${TRACER_PLUGIN_VERSION} SEQUENCER_PLUGIN_VERSION=$(SEQUENCER_PLUGIN_VERSION)" ; \
	echo "Clean and build distribution for tracer" ; \
	./gradlew tracer:plugins:clean && ./gradlew tracer:plugins:installDist -PreleaseVersion=$(TRACER_PLUGIN_VERSION) ; \
	echo "Clean and build distribution for sequencer" ; \
	./gradlew linea-besu:plugins:linea-sequencer:sequencer:clean && ./gradlew linea-besu:plugins:linea-sequencer:sequencer:installDist -PreleaseVersion=$(SEQUENCER_PLUGIN_VERSION)

assemble:
	BESU_VERSION=$(BESU_VERSION) \
	LOCAL_BESU_ZIP_PATH=$(LOCAL_BESU_ZIP_PATH) \
	LOCAL_SEQUENCER_DIST_FOLDER=$(LOCAL_SEQUENCER_DIST_FOLDER) \
	LOCAL_TRACER_DIST_FOLDER=$(LOCAL_TRACER_DIST_FOLDER) \
	./scripts/assemble-packages.sh

# When PLATFORM is unset, buildx used to pick the host platform implicitly.
# build-image.sh always passes an explicit --platform, so resolve the host
# platform here to keep that behaviour.
DOCKER_HOST_PLATFORM ?= $(shell docker version --format '{{.Server.Os}}/{{.Server.Arch}}' 2>/dev/null)
BUILD_IMAGE_PLATFORM ?= $(if $(PLATFORM),$(PLATFORM),$(DOCKER_HOST_PLATFORM))

# Delegates to the monorepo-wide build script, which is also what CI and
# `make docker-build-linea-besu-package` use. See scripts/docker/README.md.
build-image:
	../../scripts/docker/build-image.sh \
		--image-name consensys/linea-besu-package \
		--tags $(BESU_PACKAGE_TAG) \
		--dockerfile linea-besu/Dockerfile \
		--context ./tmp \
		$(if $(BUILD_IMAGE_PLATFORM),--platforms $(BUILD_IMAGE_PLATFORM)) \
		--no-registry-cache \
		--no-cache \
		--progress plain

build:
	echo "Build linea-besu-package image from source with tag=$(BESU_PACKAGE_TAG)"
	$(MAKE) build-besu
	$(MAKE) build-tracer-and-sequencer
	$(MAKE) clean
	$(MAKE) assemble
	$(MAKE) build-image

clean-build:
	$(MAKE) clean-besu-and-maven-local
	${MAKE} build

# The image itself is built by the monorepo-wide target, which mirrors
# .github/workflows/prover-build-and-publish.yml. PLATFORMS keeps the host-native
# default this target had before it was routed through scripts/docker/build-image.sh.
build-prover:
	echo "Build prover image from source with tag=$(PROVER_TAG)"
	cd ../../prover && echo "Build zkevm.bin for prover" && make zkevm/arithmetization/zkevm.bin
	echo "Build prover docker image"
	$(MAKE) -C ../.. docker-build-prover DOCKER_IMAGE_TAG=$(PROVER_TAG) PLATFORMS=$(DOCKER_HOST_PLATFORM)

build-all:
	$(MAKE) build
	$(MAKE) build-prover

run-e2e-test-with-partial-prover:
	cd ../.. && LINEA_BESU_PACKAGE_TAG=$(BESU_PACKAGE_TAG) LINEA_PROVER_TAG=$(PROVER_TAG) make start-env-with-tracing-v2-partialprover && pnpm run -F e2e test:partial-prover:local

run-e2e-test:
	cd ../.. && LINEA_BESU_PACKAGE_TAG=$(BESU_PACKAGE_TAG) make start-env-with-tracing-v2-ci && pnpm run -F e2e test:local

run-e2e-test-cleanup:
	cd ../.. && make clean-environment

clean:
	rm -fr tmp || true
