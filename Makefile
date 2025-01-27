define get_future_time
$(shell \
    OS=$$(uname); \
    if [ "$$OS" = "Linux" ]; then \
        date -d '+3 seconds' +%s; \
    elif [ "$$OS" = "Darwin" ]; then \
        date -v +3S +%s; \
    fi \
)
endef

pnpm-install:
		pnpm install

docker-pull-develop:
		L1_GENESIS_TIME=$(get_future_time) docker compose -f docker/compose.yml pull

clean-smc-folders:
		rm -f contracts/.openzeppelin/unknown-31648428.json
		rm -f contracts/.openzeppelin/unknown-1337.json

clean-local-folders:
		make clean-smc-folders
		rm -rf tmp/local/*

clean-testnet-folders:
		make clean-smc-folders
		rm -rf tmp/testnet/*

clean-environment:
		docker compose -f docker/compose.yml -f docker/compose-local-dev-traces-v2.overrides.yml --profile l1 --profile l2 --profile debug --profile staterecovery kill -s 9 || true
		docker compose -f docker/compose.yml -f docker/compose-local-dev-traces-v2.overrides.yml --profile l1 --profile l2 --profile debug --profile staterecovery down || true
		make clean-local-folders
		docker volume rm linea-local-dev linea-logs || true # ignore failure if volumes do not exist already
		docker system prune -f || true

start-l1:
		L1_GENESIS_TIME=$(get_future_time) docker compose -f docker/compose.yml -f docker/compose-local-dev.overrides.yml --profile l1 up -d

start-l2:
		docker compose -f docker/compose.yml -f docker/compose-local-dev.overrides.yml --profile l2 up -d

start-l2-blockchain-only:
		docker compose -f docker/compose.yml -f docker/compose-local-dev.overrides.yml --profile l2-bc up -d

start-whole-environment: COMPOSE_PROFILES:=l1,l2
start-whole-environment:
		# docker compose -f docker/compose.yml -f docker/compose-local-dev.overrides.yml build prover
		L1_GENESIS_TIME=$(get_future_time) COMPOSE_PROFILES=$(COMPOSE_PROFILES) docker compose -f docker/compose.yml -f docker/compose-local-dev.overrides.yml up -d


start-whole-environment-traces-v2: COMPOSE_PROFILES:=l1,l2
start-whole-environment-traces-v2:
	@if [ -z "$(L1_GENESIS_TIME)" ]; then \
		L1_GENESIS_TIME=$(get_future_time); \
	fi; \
	L1_GENESIS_TIME=$$L1_GENESIS_TIME COMPOSE_PROFILES=$(COMPOSE_PROFILES) docker compose -f docker/compose.yml -f docker/compose-local-dev-traces-v2.overrides.yml up -d

pull-all-images:
		COMPOSE_PROFILES:=l1,l2 docker compose -f docker/compose.yml -f docker/compose-local-dev-traces-v2.overrides.yml pull

pull-images-external-to-monorepo:
		docker compose -f docker/compose.yml -f docker/compose-local-dev-traces-v2.overrides.yml --profile external-to-monorepo pull

compile-contracts:
		cd contracts; \
		make compile

compile-contracts-no-cache:
		cd contracts/; \
		make force-compile

deploy-linea-rollup: L1_CONTRACT_VERSION:=6
deploy-linea-rollup:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		PRIVATE_KEY=$${DEPLOYMENT_PRIVATE_KEY:-0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80} \
		RPC_URL=http:\\localhost:8445/ \
		VERIFIER_CONTRACT_NAME=IntegrationTestTrueVerifier \
		LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH=0x072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd \
		LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER=0 \
		LINEA_ROLLUP_SECURITY_COUNCIL=0x90F79bf6EB2c4f870365E785982E1f101E93b906 \
		LINEA_ROLLUP_OPERATORS=$${LINEA_ROLLUP_OPERATORS:-0x70997970C51812dc3A010C7d01b50e0d17dc79C8,0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC} \
		LINEA_ROLLUP_RATE_LIMIT_PERIOD=86400 \
		LINEA_ROLLUP_RATE_LIMIT_AMOUNT=1000000000000000000000 \
		LINEA_ROLLUP_GENESIS_TIMESTAMP=1683325137 \
		npx ts-node local-deployments-artifacts/deployPlonkVerifierAndLineaRollupV$(L1_CONTRACT_VERSION).ts

deploy-linea-rollup-v5:
		$(MAKE) deploy-linea-rollup L1_CONTRACT_VERSION=5

deploy-linea-rollup-v6:
		$(MAKE) deploy-linea-rollup L1_CONTRACT_VERSION=6


deploy-l2messageservice:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		MESSAGE_SERVICE_CONTRACT_NAME=L2MessageService \
		PRIVATE_KEY=$${DEPLOYMENT_PRIVATE_KEY:-0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae} \
		RPC_URL=http:\\localhost:8545/ \
		L2MSGSERVICE_SECURITY_COUNCIL=0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266 \
		L2MSGSERVICE_L1L2_MESSAGE_SETTER=$${L2MSGSERVICE_L1L2_MESSAGE_SETTER:-0xd42e308fc964b71e18126df469c21b0d7bcb86cc} \
		L2MSGSERVICE_RATE_LIMIT_PERIOD=86400 \
		L2MSGSERVICE_RATE_LIMIT_AMOUNT=1000000000000000000000 \
		npx ts-node local-deployments-artifacts/deployL2MessageService.ts

deploy-token-bridge-l1:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
		RPC_URL=http:\\localhost:8445/ \
		REMOTE_CHAIN_ID=1337 \
		TOKEN_BRIDGE_L1=true \
		L1_TOKEN_BRIDGE_SECURITY_COUNCIL=0x90F79bf6EB2c4f870365E785982E1f101E93b906 \
		L2MESSAGESERVICE_ADDRESS=0xe537D669CA013d86EBeF1D64e40fC74CADC91987 \
		LINEA_ROLLUP_ADDRESS=0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9 \
		npx ts-node local-deployments-artifacts/deployBridgedTokenAndTokenBridge.ts

deploy-token-bridge-l2:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		SAVE_ADDRESS=true \
		PRIVATE_KEY=0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae \
		RPC_URL=http:\\localhost:8545/ \
		REMOTE_CHAIN_ID=31648428 \
		TOKEN_BRIDGE_L1=false \
		L2_TOKEN_BRIDGE_SECURITY_COUNCIL=0xf17f52151EbEF6C7334FAD080c5704D77216b732 \
		L2MESSAGESERVICE_ADDRESS=0xe537D669CA013d86EBeF1D64e40fC74CADC91987 \
		LINEA_ROLLUP_ADDRESS=0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9 \
		npx ts-node local-deployments-artifacts/deployBridgedTokenAndTokenBridge.ts

deploy-l1-test-erc20:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 \
		RPC_URL=http:\\localhost:8445/ \
		TEST_ERC20_L1=true \
		TEST_ERC20_NAME=TestERC20 \
		TEST_ERC20_SYMBOL=TERC20 \
		TEST_ERC20_INITIAL_SUPPLY=100000 \
		npx ts-node local-deployments-artifacts/deployTestERC20.ts

deploy-l2-test-erc20:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		PRIVATE_KEY=0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae \
		RPC_URL=http:\\localhost:8545/ \
		TEST_ERC20_L1=false \
		TEST_ERC20_NAME=TestERC20 \
		TEST_ERC20_SYMBOL=TERC20 \
		TEST_ERC20_INITIAL_SUPPLY=100000 \
		npx ts-node local-deployments-artifacts/deployTestERC20.ts

deploy-l2-evm-opcode-tester:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		PRIVATE_KEY=0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae \
		RPC_URL=http:\\localhost:8545/ \
		npx ts-node local-deployments-artifacts/deployLondonEvmTestingFramework.ts


evm-opcode-tester-execute-all-opcodes: OPCODE_TEST_CONTRACT_ADDRESS:=0x997FC3aF1F193Cbdc013060076c67A13e218980e
evm-opcode-tester-execute-all-opcodes:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		OPCODE_TEST_CONTRACT_ADDRESS=$(OPCODE_TEST_CONTRACT_ADDRESS) \
		NUMBER_OF_RUNS=3 \
		PRIVATE_KEY=0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae \
		RPC_URL=http:\\localhost:8545/ \
		npx ts-node local-deployments-artifacts/executeAllOpcodes.ts

fresh-start-l2-blockchain-only:
		make clean-environment
		make start-l2-blockchain-only

restart-shomei:
		docker compose -f docker/compose.yml -f docker/compose-local-dev.overrides.yml rm zkbesu-shomei shomei
		rm -rf tmp/local/shomei/*
		docker compose -f docker/compose.yml -f docker/compose-local-dev.overrides.yml up zkbesu-shomei shomei -d

fresh-start-all: COMPOSE_PROFILES:="l1,l2"
fresh-start-all: L1_CONTRACT_VERSION:=6
fresh-start-all:
		make clean-environment
		make start-all L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION) COMPOSE_PROFILES=$(COMPOSE_PROFILES)

fresh-start-all-traces-v2: COMPOSE_PROFILES:="l1,l2"
fresh-start-all-traces-v2: L1_CONTRACT_VERSION:=6
fresh-start-all-traces-v2:
		make clean-environment
		$(MAKE) start-all-traces-v2 L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION) COMPOSE_PROFILES=$(COMPOSE_PROFILES)

start-all: COMPOSE_PROFILES:=l1,l2
start-all: L1_CONTRACT_VERSION:=6
start-all:
		L1_GENESIS_TIME=$(get_future_time) make start-whole-environment COMPOSE_PROFILES=$(COMPOSE_PROFILES)
		make deploy-contracts L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION)

start-all-traces-v2: COMPOSE_PROFILES:="l1,l2"
start-all-traces-v2: L1_CONTRACT_VERSION:=6
start-all-traces-v2:
		L1_GENESIS_TIME=$(get_future_time) make start-whole-environment-traces-v2 COMPOSE_PROFILES=$(COMPOSE_PROFILES)
		$(MAKE) deploy-contracts L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION)

deploy-contracts: L1_CONTRACT_VERSION:=6
deploy-contracts:
	cd contracts/; \
	export L1_NONCE=$$(npx ts-node local-deployments-artifacts/get-wallet-nonce.ts --wallet-priv-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 --rpc-url http://localhost:8445) && \
	export L2_NONCE=$$(npx ts-node local-deployments-artifacts/get-wallet-nonce.ts --wallet-priv-key 0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae --rpc-url http://localhost:8545) && \
	cd .. && \
	$(MAKE) -j6 deploy-linea-rollup-v$(L1_CONTRACT_VERSION) deploy-token-bridge-l1 deploy-l1-test-erc20 deploy-l2messageservice deploy-token-bridge-l2 deploy-l2-test-erc20

deploy-contracts-minimal: L1_CONTRACT_VERSION:=6
deploy-contracts-minimal:
	cd contracts/; \
	export L1_NONCE=$$(npx ts-node local-deployments-artifacts/get-wallet-nonce.ts --wallet-priv-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 --rpc-url http://localhost:8445) && \
	export L2_NONCE=$$(npx ts-node local-deployments-artifacts/get-wallet-nonce.ts --wallet-priv-key 0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae --rpc-url http://localhost:8545) && \
	cd .. && \
	$(MAKE) -j6 deploy-linea-rollup-v$(L1_CONTRACT_VERSION) deploy-l2messageservice

fresh-start-all-staterecovery: COMPOSE_PROFILES:=l1,l2,staterecovery
fresh-start-all-staterecovery: L1_CONTRACT_VERSION:=6
fresh-start-all-staterecovery:
	make clean-environment
	make start-whole-environment-traces-v2 COMPOSE_PROFILES=$(COMPOSE_PROFILES)
	$(MAKE) deploy-contracts-minimal L1_CONTRACT_VERSION=$(L1_CONTRACT_VERSION)

fresh-start-staterecovery-for-replay-only: COMPOSE_PROFILES:=l1,staterecovery
fresh-start-staterecovery-for-replay-only:
	make clean-environment
	make start-whole-environment-traces-v2 COMPOSE_PROFILES=$(COMPOSE_PROFILES)

staterecovery-replay-from-block: L1_ROLLUP_CONTRACT_ADDRESS:=0xCf7Ed3AccA5a467e9e704C703E8D87F634fB0Fc9
staterecovery-replay-from-block: PLUGIN_STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER:=1
staterecovery-replay-from-block:
	docker compose -f docker/compose.yml down zkbesu-shomei-sr shomei-sr
	L1_ROLLUP_CONTRACT_ADDRESS=$(L1_ROLLUP_CONTRACT_ADDRESS) PLUGIN_STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER=$(PLUGIN_STATERECOVERY_OVERRIDE_START_BLOCK_NUMBER) docker compose -f docker/compose.yml up zkbesu-shomei-sr shomei-sr -d

testnet-start-l2:
		docker compose -f docker/compose.yml -f docker/compose-testnet-sync.overrides.yml --profile l2 up -d

testnet-start-l2-traces-node-only:
		docker compose -f docker/compose.yml -f docker/compose-testnet-sync.overries.yml up traces-node -d

testnet-start: start-l1 deploy-linea-rollup-v5 testnet-start-l2
testnet-restart-l2-keep-state:
		docker compose -f docker/compose.yml -f docker/compose-testnet-sync.overrides.yml rm -f -s -v sequencer traces-node coordinator
		make testnet-start-l2

testnet-restart-l2-clean-state:
		docker compose -f docker/compose.yml -f docker/compose-testnet-sync.overrides.yml rm -f -s -v sequencer traces-node coordinator
		docker volume rm testnet-data
		make clean-testnet-folders
		make testnet-start-l2

testnet-down:
		docker compose -f docker/compose.yml -f docker/compose-testnet-sync.overrides.yml --profile l1 --profile l2 down -v
		make clean-testnet-folders

stop_pid:
		if [ -f $(PID_FILE) ]; then \
			kill `cat $(PID_FILE)`; \
			echo "Stopped process with PID `cat $(PID_FILE)`"; \
			rm $(PID_FILE); \
		else \
			echo "$(PID_FILE) does not exist. No process to stop."; \
		fi

restart-l2-minimal-stack-local:
		make stop-coordinator
		make stop-traces-api
		make stop_pid PID_FILE=tmp/local/traces-app.pid
		make clean-environment
		make start-l2
		make deploy-l2messageservice-to-local
		make start-traces-api
		make start-coordinator
		# TODO: use locally built prover for faster feedback loop

stop-l2-minimal-stack-local:
		make stop-coordinator
		make stop-traces-api
		make clean-environment

start-coordinator:
		mkdir -p  tmp/local/logs
		./gradlew coordinator:app:run \
			-Dconfig.override.testL1Disabled=true \
			-Dconfig.override.traces.counters.endpoints="http://127.0.0.1:8081" \
			-Dconfig.override.traces.conflation.endpoints="http://127.0.0.1:8081" \
			-Dconfig.override.dynamic-gas-price-service.miner-gas-price-update-recipients="http://127.0.0.1:8545/,http://127.0.0.1:8645/" > tmp/local/logs/coordinator.log & echo "$$!" > tmp/local/coordinator.pid

stop-coordinator:
		make stop_pid PID_FILE=tmp/local/coordinator.pid

restart-coordinator:
		make stop-coordinator
		make start-coordinator

