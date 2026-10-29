include contracts/makefile-contracts.mk
include scripts/docker/images.mk

docker-pull-images-external-to-monorepo:
		docker compose -f docker/compose-tracing-v2-ci-extension.yml --profile external-to-monorepo pull

CLEANUP_IMAGE ?= busybox:1.37.0@sha256:bdf57e528e45e4433820e045b29b4597825a1c9e38353532d90a01445013f82e

clean-local-folders:
		$(MAKE) clean-smc-folders
		# Containers may create root-owned data; fall back to deleting it through Docker.
		@if [ -d tmp/local ]; then \
			rm -rf tmp/local/* 2>/dev/null || \
			docker run --rm --network none --user 0:0 \
				--mount "type=bind,source=$(CURDIR)/tmp/local,target=/data" \
				$(CLEANUP_IMAGE) find /data -mindepth 1 -delete; \
		fi

clean-testnet-folders:
		$(MAKE) clean-smc-folders
		rm -rf tmp/testnet/* || true # ignore failure if folders do not exist already

clean-environment:
		docker compose -f docker/compose-tracing-v2-staterecovery-extension.yml --profile l1 --profile l2 --profile debug --profile staterecovery kill -s 9 || true;
		docker compose -f docker/compose-tracing-v2-staterecovery-extension.yml --profile l1 --profile l2 --profile debug --profile staterecovery down || true;
		docker compose -p lineth down --remove-orphans --timeout 0 || true; # start-env stacks run as project "lineth"
		$(MAKE) clean-local-folders;
		$(MAKE) seed-deny-list; # truncate runtime deny-list and drop any stale lockfile from a crashed run
		docker volume rm linea-local-dev linea-logs || true; # ignore failure if volumes do not exist already
		docker image prune -f || true;

.PHONY: start-env-with-riscv

RISCV_PROVER_MODE ?= dev-mock
export RISCV_PROVER_MODE
RISCV_COMPOSE_FILE := docker/compose-tracing-v2.yml -f docker/compose-riscv.yml

start-env-with-riscv:
	@test -f docker/config/prover/riscv/prover-ray-config.$(RISCV_PROVER_MODE).toml || \
		{ echo "Unsupported RISCV_PROVER_MODE=$(RISCV_PROVER_MODE) (expected dev-mock or dev-zkvm)"; exit 1; }
	$(MAKE) start-env COMPOSE_FILE="$(RISCV_COMPOSE_FILE)" \
		L1_CONTRACT_VERSION=9 LINETH_PROTOCOL_CONTRACTS_ONLY=true \
		LINETH_L1_CONTRACT_DEPLOYMENT_TARGET=deploy-lineth-rollup-v9-stub \
		DEPLOY_FORCED_TRANSACTION_GATEWAY=false
	# The coordinator needs the deployed L1 contract, so it starts last.
	COMPOSE_PROFILES=l1,l2,riscv docker compose -f $(RISCV_COMPOSE_FILE) up -d --no-deps --wait --wait-timeout 600 prover-ray coordinator
	$(MAKE) deploy-l2-evm-opcode-tester

##
## CI e2e entry points, used by .github/actions/run-e2e-tests.
## E2E_STACK selects the stack and suite: zkevm (default) or riscv.
##
.PHONY: e2e-ci-detect-changes e2e-ci-load-images e2e-ci-pull-external-images e2e-ci-build-images \
	e2e-ci-start-env e2e-ci-start-env-with-retry e2e-ci-test

E2E_STACK ?= zkevm
E2E_IMAGE_TAG ?=
E2E_ARTIFACT_DIR ?= tmp/artifacts
E2E_BESU_PACKAGE_PROVIDED ?= false
OVERRIDE_COMPOSE_FILE ?=

ifeq ($(E2E_STACK),riscv)
E2E_START_TARGET := start-env-with-riscv
E2E_TEST_SCRIPT := test:riscv:local
E2E_TEST_TIMEOUT := $(if $(filter dev-zkvm,$(RISCV_PROVER_MODE)),40m,10m)
# Empty: keep the image tags pinned in docker/compose-riscv.yml.
E2E_FALLBACK_IMAGE_TAG :=
ifeq ($(RISCV_PROVER_MODE),dev-zkvm)
# dev-zkvm runs prover-ray built from this commit (see e2e-ci-build-images).
export LINEA_PROVER_RAY_TAG := $(E2E_IMAGE_TAG)
endif
else ifeq ($(E2E_STACK),zkevm)
E2E_START_TARGET := $(if $(OVERRIDE_COMPOSE_FILE),start-env-with-tracing-v2-ci-override OVERRIDE_COMPOSE_FILE=$(abspath $(OVERRIDE_COMPOSE_FILE)),start-env-with-tracing-v2-ci)
E2E_TEST_SCRIPT := test:local
E2E_TEST_TIMEOUT := 6m
E2E_FALLBACK_IMAGE_TAG := develop
else
$(error Unsupported E2E_STACK=$(E2E_STACK) (expected zkevm or riscv))
endif

e2e-ci-detect-changes:
	@scripts/e2e/detect-changes.sh "$${E2E_ZKEVM_FILES:-[]}" "$${E2E_RISCV_FILES:-[]}"

e2e-ci-load-images:
	@E2E_FALLBACK_IMAGE_TAG="$(E2E_FALLBACK_IMAGE_TAG)" scripts/e2e/load-image-artifacts.sh "$(E2E_ARTIFACT_DIR)"

# Skipped when besu-package is provided: its services carry the external-to-monorepo profile,
# so the pull would fetch an unpublished CI tag or overwrite the loaded artifact.
e2e-ci-pull-external-images:
ifeq ($(E2E_STACK)-$(E2E_BESU_PACKAGE_PROVIDED),zkevm-false)
	$(MAKE) retry TARGET=docker-pull-images-external-to-monorepo \
		RETRY_LIMIT=10 RETRY_TIMEOUT=10m RETRY_BACKOFF=30s
else
	@echo "Skipping external image pull for E2E_STACK=$(E2E_STACK) E2E_BESU_PACKAGE_PROVIDED=$(E2E_BESU_PACKAGE_PROVIDED)"
endif

e2e-ci-build-images:
ifeq ($(E2E_STACK)-$(RISCV_PROVER_MODE),riscv-dev-zkvm)
	$(MAKE) -C prover-ray docker-build IMAGE_TAG="$(E2E_IMAGE_TAG)"
else
	@echo "No local images to build for E2E_STACK=$(E2E_STACK)"
endif

e2e-ci-start-env:
	$(MAKE) $(E2E_START_TARGET) CLEAN_PREVIOUS_ENV=false

e2e-ci-start-env-with-retry:
	@if [ -n "$(OVERRIDE_COMPOSE_FILE)" ] && [ ! -f "$(OVERRIDE_COMPOSE_FILE)" ]; then \
		echo "Override compose file not found: $(OVERRIDE_COMPOSE_FILE)" >&2; exit 1; \
	fi
	$(MAKE) retry \
		TARGET="e2e-ci-start-env E2E_STACK=$(E2E_STACK) OVERRIDE_COMPOSE_FILE=$(if $(OVERRIDE_COMPOSE_FILE),$(abspath $(OVERRIDE_COMPOSE_FILE)))" \
		RETRY_LIMIT=10 RETRY_TIMEOUT=10m RETRY_BACKOFF=30s RETRY_CLEANUP_TARGET=clean-environment

e2e-ci-test:
	timeout $(E2E_TEST_TIMEOUT) pnpm run -F e2e $(E2E_TEST_SCRIPT)

# Ensure the runtime sequencer deny-list exists (empty) before docker compose
# bind-mounts it. Gitignored; may be mutated at test time by withDenyListAddresses.
# Also drop any stale lockfile from a crashed previous run, otherwise every
# mutation in the next run would spin for 30s before timing out.
seed-deny-list:
		: > docker/config/linea-besu-sequencer/deny-list.txt
		rm -f docker/config/linea-besu-sequencer/deny-list.txt.lock || true

start-env: COMPOSE_PROFILES:=l1,l2
start-env: CLEAN_PREVIOUS_ENV:=true
start-env: COMPOSE_FILE:=docker/compose-tracing-v2.yml
start-env: L1_CONTRACT_VERSION:=8
start-env: SKIP_CONTRACTS_DEPLOYMENT:=false
start-env: SKIP_L1_L2_NODE_HEALTH_CHECK:=false
start-env: LINETH_PROTOCOL_CONTRACTS_ONLY:=false
start-env: LINETH_L1_CONTRACT_DEPLOYMENT_TARGET:=deploy-lineth-rollup-v$(L1_CONTRACT_VERSION)
start-env:
	@if [ "$(CLEAN_PREVIOUS_ENV)" = "true" ]; then \
		$(MAKE) clean-environment; \
	else \
		echo "Starting stack reusing previous state"; \
	fi; \
	mkdir -p tmp/local; \
	$(MAKE) seed-deny-list; \
	COMPOSE_PROFILES=$(COMPOSE_PROFILES) docker compose -f $(COMPOSE_FILE) up -d; \
	while [ "$(SKIP_L1_L2_NODE_HEALTH_CHECK)" = "false" ] && \
			{ [ "$$(docker compose -f $(COMPOSE_FILE) ps -q l1-el-node | xargs -r docker inspect -f '{{.State.Health.Status}}')" != "healthy" ] || \
				[ "$$(docker compose -f $(COMPOSE_FILE) ps -q l1-cl-node | xargs -r docker inspect -f '{{.State.Health.Status}}')" != "healthy" ] || \
  			[ "$$(docker compose -f $(COMPOSE_FILE) ps -q sequencer | xargs -r docker inspect -f '{{.State.Health.Status}}')" != "healthy" ]; }; do \
  			sleep 2; \
  			echo "Checking health status of: l1-el-node, l1-cl-node and l2 sequencer..."; \
  	done
	if [ "$(SKIP_CONTRACTS_DEPLOYMENT)" = "true" ]; then \
		echo "Skipping contracts deployment"; \
	else \
		$(MAKE) deploy-contracts L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION) LINETH_PROTOCOL_CONTRACTS_ONLY=$(LINETH_PROTOCOL_CONTRACTS_ONLY) LINETH_L1_CONTRACT_DEPLOYMENT_TARGET=$(LINETH_L1_CONTRACT_DEPLOYMENT_TARGET); \
	fi

start-env-with-validium:
	$(MAKE) start-env L1_CONTRACT_VERSION=2 LINETH_COORDINATOR_DATA_AVAILABILITY=VALIDIUM LINETH_L1_CONTRACT_DEPLOYMENT_TARGET=deploy-validium

start-l1:
	$(MAKE) start-env COMPOSE_PROFILES:=l1 COMPOSE_FILE:=docker/compose-tracing-v2.yml SKIP_CONTRACTS_DEPLOYMENT:=true SKIP_L1_L2_NODE_HEALTH_CHECK:=true

start-l1-l2:
	$(MAKE) start-env COMPOSE_PROFILES:=l1,l2 COMPOSE_FILE:=docker/compose-tracing-v2.yml SKIP_CONTRACTS_DEPLOYMENT:=true SKIP_L1_L2_NODE_HEALTH_CHECK:=true

start-l2-blockchain-only:
	$(MAKE) start-env COMPOSE_PROFILES:=l2-bc COMPOSE_FILE:=docker/compose-tracing-v2.yml SKIP_CONTRACTS_DEPLOYMENT:=true SKIP_L1_L2_NODE_HEALTH_CHECK:=true

fresh-start-l2-blockchain-only:
	$(MAKE) clean-environment
	$(MAKE) start-l2-blockchain-only

##
## Creating new targets to avoid conflicts with existing targets
## Redundant targets above will cleanup once this gets merged
##
start-env-with-tracing-v2:
	$(MAKE) start-env COMPOSE_FILE=docker/compose-tracing-v2.yml LINETH_PROTOCOL_CONTRACTS_ONLY=true

## Run with tracing-v2 with partial prover
start-env-with-tracing-v2-partialprover:
	$(MAKE) start-env COMPOSE_FILE="docker/compose-tracing-v2.yml -f docker/compose-tracing-v2-partialprover-override.yml"

## Enable L2 geth node
start-env-with-tracing-v2-extra:
	$(MAKE) start-env COMPOSE_PROFILES:=l1,l2 COMPOSE_FILE:=docker/compose-tracing-v2-extra-extension.yml LINETH_PROTOCOL_CONTRACTS_ONLY=true LINETH_COORDINATOR_DISABLE_TYPE2_STATE_PROOF_PROVIDER=false LINETH_COORDINATOR_SIGNER_TYPE=web3signer

start-env-with-tracing-v2-ci:
	$(MAKE) start-env COMPOSE_FILE=docker/compose-tracing-v2-ci-extension.yml LINETH_COORDINATOR_DISABLE_TYPE2_STATE_PROOF_PROVIDER=false LINETH_COORDINATOR_SIGNER_TYPE=web3signer

## Start the tracing-v2 CI stack with a custom override compose file layered on top of the CI extension.
## Pass the override via OVERRIDE_COMPOSE_FILE (absolute, or relative to this directory)
start-env-with-tracing-v2-ci-override:
ifndef OVERRIDE_COMPOSE_FILE
	$(error OVERRIDE_COMPOSE_FILE is required. run: OVERRIDE_COMPOSE_FILE=<path-to-override.yml> make start-env-with-tracing-v2-ci-override)
endif
	$(MAKE) start-env COMPOSE_FILE="docker/compose-tracing-v2-ci-extension.yml -f $(OVERRIDE_COMPOSE_FILE)" LINETH_COORDINATOR_DISABLE_TYPE2_STATE_PROOF_PROVIDER=false LINETH_COORDINATOR_SIGNER_TYPE=web3signer

start-env-with-validium-and-tracing-v2-ci:
	$(MAKE) start-env-with-validium COMPOSE_FILE=docker/compose-tracing-v2-ci-extension.yml LINETH_COORDINATOR_DISABLE_TYPE2_STATE_PROOF_PROVIDER=false LINETH_COORDINATOR_SIGNER_TYPE=web3signer

start-env-with-staterecovery: COMPOSE_PROFILES:=l1,l2,staterecovery
start-env-with-staterecovery: L1_CONTRACT_VERSION:=6
start-env-with-staterecovery:
	$(MAKE) start-env COMPOSE_FILE=docker/compose-tracing-v2-staterecovery-extension.yml LINETH_PROTOCOL_CONTRACTS_ONLY=true L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION) COMPOSE_PROFILES=$(COMPOSE_PROFILES)

staterecovery-replay-from-block: L1_ROLLUP_CONTRACT_ADDRESS:=0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9
staterecovery-replay-from-block: STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER:=1
staterecovery-replay-from-block:
	docker compose -f docker/compose-tracing-v2-staterecovery-extension.yml down zkbesu-shomei-sr shomei-sr
	L1_ROLLUP_CONTRACT_ADDRESS=$(L1_ROLLUP_CONTRACT_ADDRESS) STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER=$(STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER) docker compose -f docker/compose-tracing-v2-staterecovery-extension.yml up zkbesu-shomei-sr shomei-sr -d

# Retries TARGET up to RETRY_LIMIT times, aborting each attempt after
# RETRY_TIMEOUT. Uses perl's alarm+exec instead of GNU coreutils `timeout`,
# which isn't installed by default on macOS. Optionally runs
# RETRY_CLEANUP_TARGET between failed attempts (e.g. to reset docker state).
retry: RETRY_LIMIT:=5
retry: RETRY_TIMEOUT:=10m
retry: RETRY_BACKOFF:=10s
retry:
	@if [ -z "$(TARGET)" ]; then \
		echo "Usage: make retry TARGET=<target> [RETRY_LIMIT=n] [RETRY_TIMEOUT=Xm] [RETRY_BACKOFF=Xs] [RETRY_CLEANUP_TARGET=<target>]"; \
		exit 1; \
	fi; \
	to_seconds() { \
		case "$$1" in \
			*h) echo $$(( $${1%h} * 3600 )) ;; \
			*m) echo $$(( $${1%m} * 60 )) ;; \
			*s) echo $${1%s} ;; \
			*) echo "$$1" ;; \
		esac; \
	}; \
	timeout_secs=$$(to_seconds $(RETRY_TIMEOUT)); \
	backoff_secs=$$(to_seconds $(RETRY_BACKOFF)); \
	attempt=1; \
	until perl -e 'alarm shift; exec @ARGV' $$timeout_secs $(MAKE) $(TARGET); do \
		if [ $$attempt -ge $(RETRY_LIMIT) ]; then \
			echo "Retry limit ($(RETRY_LIMIT)) reached for target: $(TARGET)"; \
			exit 1; \
		fi; \
		echo "Attempt $$attempt of '$(TARGET)' failed. Retrying in $(RETRY_BACKOFF)..."; \
		if [ -n "$(RETRY_CLEANUP_TARGET)" ]; then \
			$(MAKE) $(RETRY_CLEANUP_TARGET) || true; \
		fi; \
		attempt=$$((attempt + 1)); \
		sleep $$backoff_secs; \
	done

stop_pid:
		if [ -f $(PID_FILE) ]; then \
			kill `cat $(PID_FILE)`; \
			echo "Stopped process with PID `cat $(PID_FILE)`"; \
			rm $(PID_FILE); \
		else \
			echo "$(PID_FILE) does not exist. No process to stop."; \
		fi
