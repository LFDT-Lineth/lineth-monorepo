# Local simulation of the CI Docker image builds.
#
# Each target mirrors the matching .github/workflows/<name>-build-and-publish.yml:
# same pre-build step, same Dockerfile / context / build-args / build-contexts,
# run through the same scripts/docker/build-image.sh that CI uses.
#
#   make docker-build-list                          # what can be built
#   make docker-build-coordinator                   # build consensys/linea-coordinator:local
#   make docker-build-coordinator DOCKER_IMAGE_TAG=mytag
#   make docker-build-prover DRY_RUN=true           # print the buildx command only
#   make docker-build-maru SKIP_PREBUILD=true       # reuse the previous gradle dist
#   make docker-build-all
#   make docker-build-linea-besu-package            # slow: builds Besu from source
#
# See scripts/docker/README.md.

DOCKER_IMAGE_TAG ?= local
# CI provisions a docker-container builder; do the same locally so that
# multi-platform builds and registry cache import behave identically.
DOCKER_BUILDER ?= linea-local
# Empty means: let the script pick the CI default (linux/amd64 for local builds).
PLATFORMS ?=
# Off by default so a local build needs neither network nor registry credentials.
REGISTRY_CACHE ?= false
DRY_RUN ?= false
SKIP_PREBUILD ?= false
NODE_VERSION ?= $(shell tr -d '[:space:]' < .nvmrc)
POSTMAN_NATIVE_LIBS_RELEASE_TAG ?= blob-libs-v3.0.1
# linea-besu-package image metadata, mirroring .github/actions/linea-besu-package/
# compute-metadata (build_date there is `date --rfc-3339=date`, i.e. date only).
LINEA_BESU_VCS_REF ?= $(shell git rev-parse HEAD)
LINEA_BESU_BUILD_DATE ?= $(shell date -u +%Y-%m-%d)

DOCKER_BUILD := scripts/docker/build-image.sh \
	--tags $(DOCKER_IMAGE_TAG) \
	$(if $(DOCKER_BUILDER),--builder $(DOCKER_BUILDER)) \
	$(if $(filter true,$(REGISTRY_CACHE)),--registry-cache,--no-registry-cache) \
	$(if $(PLATFORMS),--platforms $(PLATFORMS)) \
	$(if $(filter true,$(DRY_RUN)),--dry-run)

# Pre-build steps (gradle dists) can be skipped with SKIP_PREBUILD=true when the
# artefacts are already in place — the Docker build itself is what you iterate on.
# DRY_RUN also skips them, so printing the buildx command never triggers a compile.
define prebuild
	@if [ "$(SKIP_PREBUILD)" = "true" ] || [ "$(DRY_RUN)" = "true" ]; then \
		echo "skipping pre-build: $(1)"; \
	else \
		$(1); \
	fi
endef

DOCKER_IMAGE_TARGETS := \
	docker-build-linea-besu-package \
	docker-build-coordinator \
	docker-build-transaction-exclusion-api \
	docker-build-maru \
	docker-build-postman \
	docker-build-prover \
	docker-build-native-yield-automation-service \
	docker-build-lido-governance-monitor

.PHONY: $(DOCKER_IMAGE_TARGETS) docker-build-all docker-build-list

docker-build-list:
	@echo "Available image targets (tag: $(DOCKER_IMAGE_TAG)):"
	@for t in $(DOCKER_IMAGE_TARGETS); do echo "  make $$t"; done

docker-build-all: $(DOCKER_IMAGE_TARGETS)

# .github/workflows/coordinator-build-and-publish.yml
docker-build-coordinator:
	$(call prebuild,./gradlew coordinator:app:installDist)
	$(DOCKER_BUILD) \
		--image-name consensys/linea-coordinator \
		--dockerfile ./coordinator/Dockerfile \
		--context . \
		--build-context libs=./coordinator/app/build/install/coordinator/lib

# .github/workflows/transaction-exclusion-api-build-and-publish.yml
docker-build-transaction-exclusion-api:
	$(call prebuild,./gradlew transaction-exclusion-api:app:installDist)
	$(DOCKER_BUILD) \
		--image-name consensys/linea-transaction-exclusion-api \
		--dockerfile ./transaction-exclusion-api/Dockerfile \
		--context . \
		--build-context libs=./transaction-exclusion-api/app/build/install/transaction-exclusion-api/lib

# .github/workflows/maru-build-and-publish.yml
docker-build-maru:
	$(call prebuild,./gradlew :maru:app:installDist)
	$(DOCKER_BUILD) \
		--image-name consensys/maru \
		--dockerfile maru/app/Dockerfile \
		--context maru/app \
		--build-context libs=./maru/app/build/install/app/lib/ \
		--build-context maru=./maru/app/build/libs/

# .github/workflows/postman-build-and-publish.yml
docker-build-postman:
	$(DOCKER_BUILD) \
		--image-name consensys/linea-postman \
		--dockerfile ./postman/Dockerfile \
		--context . \
		--build-arg NATIVE_LIBS_RELEASE_TAG=$(POSTMAN_NATIVE_LIBS_RELEASE_TAG) \
		--build-arg NODE_VERSION=$(NODE_VERSION)

# .github/workflows/prover-build-and-publish.yml
docker-build-prover:
	$(DOCKER_BUILD) \
		--image-name consensys/linea-prover \
		--dockerfile ./prover/Dockerfile \
		--context . \
		--build-context prover=prover/ \
		--build-arg 'RUSTFLAGS="-C target-cpu=x86-64-v3"'

# .github/workflows/native-yield-automation-service-build-and-publish.yml
docker-build-native-yield-automation-service:
	$(DOCKER_BUILD) \
		--image-name consensys/linea-native-yield-automation-service \
		--dockerfile ./operations/native-yield/automation-service/Dockerfile \
		--context . \
		--build-arg NODE_VERSION=$(NODE_VERSION)

# .github/workflows/lido-governance-monitor-build-and-publish.yml
docker-build-lido-governance-monitor:
	$(DOCKER_BUILD) \
		--image-name consensys/linea-lido-governance-monitor \
		--dockerfile ./operations/native-yield/lido-governance-monitor/Dockerfile \
		--context . \
		--build-arg NODE_VERSION=$(NODE_VERSION)

# .github/workflows/reusable-linea-besu-package-build-test-push.yml
#
# The pre-build chain is the same one CI runs through
# .github/actions/linea-besu-package/build-plugins-and-assemble: compile Besu from
# source, build the tracer and sequencer plugins, then assemble them together with
# the downloaded staterecovery/shomei plugins into linea-besu/package/tmp.
#
# CI then does `mv ./tmp/besu ./linea-besu` and builds with linea-besu/ as context.
# We keep tmp/ as the context instead: the Dockerfile's only context dependency is
# `COPY besu /opt/besu/`, so the image is identical, and every generated file stays
# inside tmp/, which `make -C linea-besu/package clean` already removes rather than
# leaving an untracked besu/ inside the git-tracked linea-besu/ directory.
#
# The `-with-fleet` variant is not reproduced: it needs a token for the private
# Consensys/besu-fleet-plugin repository.
docker-build-linea-besu-package:
	$(call prebuild,$(MAKE) -C linea-besu/package build-besu build-tracer-and-sequencer clean assemble)
	$(DOCKER_BUILD) \
		--image-name consensys/linea-besu-package \
		--dockerfile linea-besu/package/linea-besu/Dockerfile \
		--context linea-besu/package/tmp \
		--build-arg VERSION=$(DOCKER_IMAGE_TAG) \
		--build-arg VCS_REF=$(LINEA_BESU_VCS_REF) \
		--build-arg BUILD_DATE=$(LINEA_BESU_BUILD_DATE)
