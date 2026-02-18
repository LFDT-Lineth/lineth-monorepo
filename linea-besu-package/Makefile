include versions.env

TAG ?= local
PLATFORM ?=
EXPECTED_TRACES_API_VERSION ?= ${LINEA_TRACER_PLUGIN_VERSION}
BESU_VERSION ?= $(shell grep -E '^\s*besu\s*=' ../gradle/libs.versions.toml | sed -n 's/.*"\([^"]*\)".*/\1/p')
ARITHMETIZATION_VERSION ?= $(shell grep -E '^\s*arithmetization\s*=' ../gradle/releases.versions.toml | sed -n 's/.*"\([^"]*\)".*/\1/p')
TRACER_PLUGIN_VERSION ?= $(ARITHMETIZATION_VERSION)-$(shell git rev-parse --short=7 HEAD)
SEQUENCER_PLUGIN_VERSION ?= $(ARITHMETIZATION_VERSION)-$(shell git rev-parse --short=7 HEAD)
LOCAL_SEQUENCER_ZIP_FOLDER ?= $(abspath ../besu-plugins/linea-sequencer/sequencer/build/distributions)
LOCAL_TRACER_ZIP_FOLDER ?= $(abspath ../tracer/plugins/build/distributions)
LOCAL_TRACER_ZIP_PATH ?= $(LOCAL_TRACER_ZIP_FOLDER)/linea-tracer-$(TRACER_PLUGIN_VERSION).zip
LOCAL_SEQUENCER_ZIP_PATH ?= $(LOCAL_SEQUENCER_ZIP_FOLDER)/linea-sequencer-$(SEQUENCER_PLUGIN_VERSION).zip

build-tracer-and-sequencer-local:
	cd ..; \
	echo "Build versions: ARITHMETIZATION_VERSION=$(ARITHMETIZATION_VERSION) TRACER_PLUGIN_VERSION=${TRACER_PLUGIN_VERSION} SEQUENCER_PLUGIN_VERSION=$(SEQUENCER_PLUGIN_VERSION)" ; \
	echo "Clean and build zip distribution for tracer" ; \
	./gradlew :tracer:clean && ./gradlew :tracer:artifacts -PreleaseVersion=$(TRACER_PLUGIN_VERSION) ; \
	echo "Clean and build zip distribution for sequencer" ; \
	./gradlew besu-plugins:linea-sequencer:sequencer:clean && ./gradlew besu-plugins:linea-sequencer:sequencer:artifacts -PreleaseVersion=$(SEQUENCER_PLUGIN_VERSION)

assemble:
	./scripts/assemble-packages.sh

assemble-local:
	BESU_VERSION=$(BESU_VERSION) LOCAL_SEQUENCER_ZIP_PATH=$(LOCAL_SEQUENCER_ZIP_PATH) LOCAL_TRACER_ZIP_PATH=$(LOCAL_TRACER_ZIP_PATH) ./scripts/assemble-packages.sh

build:
	$(MAKE) assemble
	$(MAKE) build-image

build-local:
	$(MAKE) build-tracer-and-sequencer-local
	$(MAKE) assemble-local
	$(MAKE) build-image

build-image:
	echo "IMAGE TAG=$(TAG)"
	echo "PLATFORM=$(PLATFORM)"
	if [ "$(PLATFORM)" = "" ]; then \
		echo "Building image for default platform"; \
		docker buildx build --file linea-besu/Dockerfile --tag consensys/linea-besu-package:$(TAG) ./tmp --progress=plain --no-cache --load; \
	else \
		echo "Building image for $(PLATFORM)"; \
		docker buildx build --file linea-besu/Dockerfile --platform $(PLATFORM) --tag consensys/linea-besu-package:$(TAG) ./tmp --progress=plain --no-cache --load; \
	fi

run-e2e-test:
	echo "EXPECTED_TRACES_API_VERSION=$(EXPECTED_TRACES_API_VERSION)"; \
	if [ "$(shell uname)" = "Linux" ]; then \
		sed -i'' 's/^\(expected-traces-api-version[ ]*=[ ]*\).*/\1"$(EXPECTED_TRACES_API_VERSION)"/' ../config/coordinator/coordinator-config-v2.toml; \
	elif [ "$(shell uname)" = "Darwin" ]; then \
		sed -i '' 's/^\(expected-traces-api-version[ ]*=[ ]*\).*/\1"$(EXPECTED_TRACES_API_VERSION)"/' ../config/coordinator/coordinator-config-v2.toml; \
	fi
	cd .. && BESU_PACKAGE_TAG=$(TAG) make start-env-with-tracing-v2-ci && pnpm run -F e2e test:local

run-e2e-test-with-arithmetization-version:
	EXPECTED_TRACES_API_VERSION=$(ARITHMETIZATION_VERSION) $(MAKE) run-e2e-test

run-e2e-test-cleanup:
	cd .. && make clean-environment

clean:
	rm -fr tmp || true
