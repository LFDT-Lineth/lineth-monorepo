include makefile-contracts.mk

docker-pull-images-external-to-monorepo:
		docker compose -f docker/compose-tracing-v2-ci-extension.yml --profile external-to-monorepo pull

clean-local-folders:
		make clean-smc-folders
		rm -rf tmp/local/* || true # ignore failure if folders do not exist already

clean-testnet-folders:
		make clean-smc-folders
		rm -rf tmp/testnet/* || true # ignore failure if folders do not exist already

clean-environment:
		docker compose -f docker/compose-tracing-v2-ci-extension.yml -f docker/compose-tracing-v2-staterecovery-extension.yml --profile l1 --profile l2 --profile debug --profile staterecovery kill -s 9 || true;
		docker compose -f docker/compose-tracing-v2-ci-extension.yml -f docker/compose-tracing-v2-staterecovery-extension.yml --profile l1 --profile l2 --profile debug --profile staterecovery down || true;
		make clean-local-folders;
		docker volume rm linea-local-dev linea-logs || true; # ignore failure if volumes do not exist already
		docker system prune -f || true;

start-env: COMPOSE_PROFILES:=l1,l2
start-env: CLEAN_PREVIOUS_ENV:=true
start-env: COMPOSE_FILE:=docker/compose-tracing-v2.yml
start-env: L1_CONTRACT_VERSION:=6
start-env: SKIP_CONTRACTS_DEPLOYMENT:=false
start-env: LINEA_PROTOCOL_CONTRACTS_ONLY:=false
start-env:
	@if [ "$(CLEAN_PREVIOUS_ENV)" = "true" ]; then \
		$(MAKE) clean-environment; \
	else \
		echo "Starting stack reusing previous state"; \
	fi; \
	mkdir -p tmp/local; \
	COMPOSE_PROFILES=$(COMPOSE_PROFILES) docker compose -f $(COMPOSE_FILE) up -d; \
	while [ "$$(docker compose -f $(COMPOSE_FILE) ps -q l1-el-node | xargs docker inspect -f '{{.State.Health.Status}}')" != "healthy" ] || \
  			[ "$$(docker compose -f $(COMPOSE_FILE) ps -q sequencer | xargs docker inspect -f '{{.State.Health.Status}}')" != "healthy" ]; do \
  			sleep 2; \
  			echo "Checking health status of l1-el-node and sequencer..."; \
  	done
	if [ "$(SKIP_CONTRACTS_DEPLOYMENT)" = "true" ]; then \
		echo "Skipping contracts deployment"; \
	else \
		$(MAKE) deploy-contracts L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION) LINEA_PROTOCOL_CONTRACTS_ONLY=$(LINEA_PROTOCOL_CONTRACTS_ONLY); \
	fi

start-l1:
	command start-env COMPOSE_PROFILES:=l1 COMPOSE_FILE:=docker/compose-tracing-v2.yml SKIP_CONTRACTS_DEPLOYMENT:=true

start-l2:
	command start-env COMPOSE_PROFILES:=l2 COMPOSE_FILE:=docker/compose-tracing-v2.yml SKIP_CONTRACTS_DEPLOYMENT:=true

start-l2-blockchain-only:
	command start-env COMPOSE_PROFILES:=l2-bc COMPOSE_FILE:=docker/compose-tracing-v2.yml SKIP_CONTRACTS_DEPLOYMENT:=true

fresh-start-l2-blockchain-only:
		make clean-environment
		make start-l2-blockchain-only

##
## Creating new targets to avoid conflicts with existing targets
## Redundant targets above will cleanup once this get's merged
##
start-env-with-tracing-v2:
	make start-env COMPOSE_FILE=docker/compose-tracing-v2.yml LINEA_PROTOCOL_CONTRACTS_ONLY=true

start-env-with-tracing-v2-ci:
	make start-env COMPOSE_FILE=docker/compose-tracing-v2-ci-extension.yml DISABLE_JSON_RPC_PRICING_PROPAGATION=false

start-env-with-staterecovery: COMPOSE_PROFILES:=l1,l2,staterecovery
start-env-with-staterecovery: L1_CONTRACT_VERSION:=6
start-env-with-staterecovery:
	make start-env COMPOSE_FILE=docker/compose-tracing-v2-staterecovery-extension.yml LINEA_PROTOCOL_CONTRACTS_ONLY=true L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION) COMPOSE_PROFILES=$(COMPOSE_PROFILES)

staterecovery-replay-from-block: L1_ROLLUP_CONTRACT_ADDRESS:=0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9
staterecovery-replay-from-block: STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER:=1
staterecovery-replay-from-block:
	docker compose -f docker/compose-tracing-v2-staterecovery-extension.yml down zkbesu-shomei-sr shomei-sr
	L1_ROLLUP_CONTRACT_ADDRESS=$(L1_ROLLUP_CONTRACT_ADDRESS) STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER=$(STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER) docker compose -f docker/compose-tracing-v2-staterecovery-extension.yml up zkbesu-shomei-sr shomei-sr -d

stop_pid:
		if [ -f $(PID_FILE) ]; then \
			kill `cat $(PID_FILE)`; \
			echo "Stopped process with PID `cat $(PID_FILE)`"; \
			rm $(PID_FILE); \
		else \
			echo "$(PID_FILE) does not exist. No process to stop."; \
		fi


