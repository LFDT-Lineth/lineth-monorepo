pnpm-install:
		pnpm install

clean-smc-folders:
		rm -f contracts/.openzeppelin/unknown-31648428.json
		rm -f contracts/.openzeppelin/unknown-1337.json

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
		npx ts-node local-deployments-artifacts/deployL2MessageServiceV1.ts

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
		npx ts-node local-deployments-artifacts/deployBridgedTokenAndTokenBridgeV1.ts

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
		npx ts-node local-deployments-artifacts/deployBridgedTokenAndTokenBridgeV1.ts

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

deploy-contracts: L1_CONTRACT_VERSION:=6
deploy-contracts: LINEA_PROTOCOL_CONTRACTS_ONLY:=false
deploy-contracts:
	cd contracts/; \
	export L1_NONCE=$$(npx ts-node local-deployments-artifacts/get-wallet-nonce.ts --wallet-priv-key 0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80 --rpc-url http://localhost:8445) && \
	export L2_NONCE=$$(npx ts-node local-deployments-artifacts/get-wallet-nonce.ts --wallet-priv-key 0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae --rpc-url http://localhost:8545) && \
	cd .. && \
	if [ "$(LINEA_PROTOCOL_CONTRACTS_ONLY)" = "false" ]; then \
		$(MAKE) -j6 deploy-linea-rollup-v$(L1_CONTRACT_VERSION) deploy-token-bridge-l1 deploy-l1-test-erc20 deploy-l2messageservice deploy-token-bridge-l2 deploy-l2-test-erc20; \
	else \
		$(MAKE) -j6 deploy-linea-rollup-v$(L1_CONTRACT_VERSION) deploy-l2messageservice; \
	fi


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

deploy-l2-scenario-testing-proxy:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		cd contracts/; \
		PRIVATE_KEY=0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae \
		RPC_URL=http:\\localhost:8545/ \
		npx ts-node local-deployments-artifacts/deployLineaScenarioDelegatingProxy.ts

execute-scenario-testing-proxy-scenario: LINEA_SCENARIO_DELEGATING_PROXY_ADDRESS:=0x2f6dAaF8A81AB675fbD37Ca6Ed5b72cf86237453
execute-scenario-testing-proxy-scenario:
		# WARNING: FOR LOCAL DEV ONLY - DO NOT REUSE THESE KEYS ELSEWHERE
		# GAS_LIMIT=452500 will cause it to fail
		cd contracts/; \
		LINEA_SCENARIO_DELEGATING_PROXY_ADDRESS=$(LINEA_SCENARIO_DELEGATING_PROXY_ADDRESS) \
		NUMBER_OF_LOOPS=10000000 \
		LINEA_SCENARIO=1 \
		GAS_LIMIT=452500 \
		PRIVATE_KEY=0x1dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae \
		RPC_URL=http:\\localhost:8545/ \
		npx ts-node local-deployments-artifacts/executeLineaScenarioDelegatingProxyScenario.ts

