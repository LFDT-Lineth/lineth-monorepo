# Linea Deployment Scripts
<br />

This document aims to explain how to get started with deploying the Linea deployment scripts. There are several ways the scripts can be executed dependent on: 
- If you're storing deployment variables in an environment file (.env)
- If you plan to deploy an individual script which will deploy a single contract.
- If you plan to deploy a chained deployment script that will include multiple contracts.

<br />
Running the script with an .env file set, you will need to make sure that the correct variables are set in the .env file, considering the network that you're deploying on. In this way when the script is being run, it will take the variables it needs to execute the script from that .env file. <br />
<br />

Running the script without an .env file will require you to place the variables as command-line arguments.
The command-line arguments will create or replace existing .env (only in memory) environment variables. If the variables are provided in the terminal as command-line arguments, they will have priority over the same variables if they are defined in the .env file. These need not exist in the .env file.

Furthermore, you can also specify a general set of variables in the .env file (SAVE_ADDRESS, VERIFY_CONTRACT, SEPOLIA_PRIVATE_KEY, LINEA_SEPOLIA_PRIVATE_KEY, MAINNET_PRIVATE_KEY, LINEA_MAINNET_PRIVATE_KEY, ETHERSCAN_API_KEY, LINEASCAN_API_KEY, INFURA_API_KEY) and provide only the script-specific variables as command-line arguments, when you run each script.

Setting `SAVE_ADDRESS=true` will make the script write a file in the deployments/<network_name>/ folder which stores the contract address, ABI and transaction hash.
<br />
Setting `VERIFY_CONTRACT=true` will start the verifying stage after the contract is deployed, provided that there is a `ETHERSCAN_API_KEY` or `LINEASCAN_API_KEY` available in the .env or provided as CLI argument.

<br />

## Network Specific Variables

Dependent on which network you are using, a specific network private key needs to be used, as well as the corresponding API Key or RPC URL.  Also, dependent on which network you choose, the block explorer used could be different, so the block explorer parameter name might need to be adjusted.  The following table highlights which private key variable will be used per network. Please use the variable that pertains to the network. e.g. for `linea_sepolia` use `LINEA_SEPOLIA_PRIVATE_KEY` (`LINEA_SEPOLIA_PRIVATE_KEY=<key> INFURA_API_KEY=<key>`)  

| Network       | Private key parameter name   | API Key / RPC URL | Block explorer parameter name |
| ------------- | ----------------- | ---- | ----------------- | 
| sepolia    | SEPOLIA_PRIVATE_KEY    | INFURA_API_KEY  | ETHERSCAN_API_KEY |
| linea_sepolia | LINEA_SEPOLIA_PRIVATE_KEY   | INFURA_API_KEY  | LINEASCAN_API_KEY |
| mainnet   | MAINNET_PRIVATE_KEY | INFURA_API_KEY | ETHERSCAN_API_KEY |
| linea_mainnet | LINEA_MAINNET_PRIVATE_KEY |  INFURA_API_KEY  | LINEASCAN_API_KEY |
| custom    | CUSTOM_PRIVATE_KEY | CUSTOM_BLOCKCHAIN_URL | ETHERSCAN_API_KEY |
| zkevm_dev | PRIVATE_KEY | BLOCKCHAIN_NODE or L2_BLOCKCHAIN_NODE | n/a |

<br />

## Generalized Command Format

```shell
<possible CLI environment arguments> npx hardhat deploy --network sepolia --tags <contract tags, comma delimitted list>
```

<br />
<br />

## Order of Precedence

 When deploying, if required variables such as deployed contract addresses are not defined in the .env or provided as CLI arguments, the script will look and check if it can use the addresses stored in the deployments/<network_name>/ folder. 
 <br />
 The order of priority (unless specified otherwise) will be:
 - CLI arguments, 
 - .env variables ,
 - deployments/<network_name>/

## Deployments and their parameters
### Verifier
<br />

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name             | Required | Input Value | Description |
| -------------------------- | -------- | ---------- | ----------- |
| SAVE_ADDRESS       | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash] |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| PLONKVERIFIER_NAME | true  | string | The name of the PlonkVerifier contract that should be deployed |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags PlonkVerifier
```

Base command with cli arguments:

```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true SEPOLIA_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> PLONKVERIFIER_NAME=PlonkVerifierForMultiTypeDataAggregation npx hardhat deploy --network sepolia --tags PlonkVerifier
```

(make sure to replace `<key>` with actual values)
<br />
<br />

### LineaRollup
<br />

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| SAVE_ADDRESS       | false    | true\|false | Saves file with deployment details [address, ABI, transaction hash] |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network.|
| LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH   | true      | bytes | Initial State Root Hash |
| LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER   | true      | uint256 | Initial L2 Block Number |
| LINEA_ROLLUP_SECURITY_COUNCIL  | true      | address | L1 Security Council Address |
| LINEA_ROLLUP_OPERATORS     | true      | address | L1 Operators Addresses (comma-delimited if multiple) |
| LINEA_ROLLUP_RATE_LIMIT_PERIOD     | true  | uint256   | L1 Rate Limit Period |
| LINEA_ROLLUP_RATE_LIMIT_AMOUNT     | true  | uint256   | L1 Rate Limit Amount |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags LineaRollup
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true SEPOLIA_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH=<bytes> LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER=<value> LINEA_ROLLUP_SECURITY_COUNCIL=<address> LINEA_ROLLUP_OPERATORS=<address> LINEA_ROLLUP_RATE_LIMIT_PERIOD=<value> LINEA_ROLLUP_RATE_LIMIT_AMOUNT=<value> npx hardhat deploy --network sepolia --tags LineaRollup
```

(make sure to replace `<value>` `<key>` `<bytes>` `<address>` with actual values).

<br />
<br />

### Linea Voyage XP Token
<br />


Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| SAVE_ADDRESS       | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash] |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| LINEA_VOYAGE_XP_ADMIN_ADDRESS | true     | address | Admin and minter address  |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags LineaVoyageXPToken
```

### Timelock
<br />


Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| SAVE_ADDRESS       | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash] |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| TIMELOCK_PROPOSERS | true     | address | Timelock Proposers address |
| TIMELOCK_EXECUTORS | true     | address | Timelock Executors address |
| TIMELOCK_ADMIN_ADDRESS | true     | address | Timelock Admin address |
| MIN_DELAY | true      | uint256 | Timelock Minimum Delay |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags Timelock
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true SEPOLIA_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> TIMELOCK_PROPOSERS=<address> TIMELOCK_EXECUTORS=<address> TIMELOCK_ADMIN_ADDRESS=<address> MIN_DELAY=<value> npx hardhat deploy --network sepolia --tags Timelock
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />
<br />

### L2MessageService
<br />

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| SAVE_ADDRESS       | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash] |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2MSGSERVICE_SECURITY_COUNCIL | true   | address | L2 Security council address |
| L2MSGSERVICE_L1L2_MESSAGE_SETTER  | true  |  address | L1L2 Message Setter address on L2 |
| L2MSGSERVICE_RATE_LIMIT_PERIOD    | true  |  uint256 | L2 Rate Limit Period |
| L2MSGSERVICE_RATE_LIMIT_AMOUNT    | true  |  uint256 | L2 Rate Limit Amount |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags L2MessageService
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true SEPOLIA_PRIVATE_KEY=<key> LINEASCAN_API_KEY=<key> INFURA_API_KEY=<key> L2MSGSERVICE_SECURITY_COUNCIL=<address> L2MSGSERVICE_L1L2_MESSAGE_SETTER=<address>  L2MSGSERVICE_RATE_LIMIT_PERIOD=<value> L2MSGSERVICE_RATE_LIMIT_AMOUNT=<value> npx hardhat deploy --network linea_sepolia --tags L2MessageService
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />
<br />

### BridgedToken
<br />

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| SAVE_ADDRESS          | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash]. |
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contract. |
| \**PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags BridgedToken
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true LINEASCAN_API_KEY=<key> LINEA_SEPOLIA_PRIVATE_KEY=<key> INFURA_API_KEY=<key> npx hardhat deploy --network linea_sepolia --tags BridgedToken
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />
<br />

### CustomBridgedToken
<br />

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| CUSTOMTOKENBRIDGE_NAME | true    |string| Token's name |
| CUSTOMTOKENBRIDGE_SYMBOL | true    |string| Token's symbol |
| CUSTOMTOKENBRIDGE_DECIMALS | true    |uint256| Token's decimals |
| CUSTOMTOKENBRIDGE_BRIDGE_ADDRESS | true    |address| Token bridge's address|
| SAVE_ADDRESS          | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash]. |
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contract. |
| \**PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags CustomBridgedToken
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true LINEASCAN_API_KEY=<key> LINEA_SEPOLIA_PRIVATE_KEY=<key> INFURA_API_KEY=<key> CUSTOMTOKENBRIDGE_NAME=<name> CUSTOMTOKENBRIDGE_SYMBOL=<symbol> CUSTOMTOKENBRIDGE_DECIMALS=<decimals> CUSTOMTOKENBRIDGE_BRIDGE_ADDRESS=<address> npx hardhat deploy --network linea_sepolia --tags CustomBridgedToken
```

(make sure to replace `<key>` `<address>` `<name>` `<symbol>` `<decimals>` with actual values)

<br />
<br />

### TokenBridge
<br />

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| SAVE_ADDRESS          | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash]. |
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contract. |
| \**PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2_MESSAGE_SERVICE_ADDRESS    | true  | address   | L2 Message Service address used when deploying TokenBridge.    |
| LINEA_ROLLUP_ADDRESS         | true    | address       | L1 Rollup address used when deploying Token Bridge.   |
| REMOTE_CHAIN_ID       | true      |   uint256     | ChainID of the remote (target) network |
| TOKEN_BRIDGE_L1       | false     |true\|false| If Token Bridge is deployed on L1, TOKEN_BRIDGE_L1 should be set to `true`. Otherwise it should be `false`|
| L1_RESERVED_TOKEN_ADDRESSES | false   | address   | If TOKEN_BRIDGE_L1=true, then L1_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|
| L2_RESERVED_TOKEN_ADDRESSES | false   | address   | If TOKEN_BRIDGE_L1=false, then L2_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags TokenBridge
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true LINEASCAN_API_KEY=<key> LINEA_SEPOLIA_PRIVATE_KEY=<key> INFURA_API_KEY=<key> REMOTE_CHAIN_ID=<uint256> TOKEN_BRIDGE_L1=true L1_RESERVED_TOKEN_ADDRESSES=<address> L2_MESSAGE_SERVICE_ADDRESS=<address> LINEA_ROLLUP_ADDRESS=<address> npx hardhat deploy --network linea_sepolia --tags TokenBridge
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />
<br />

## Chained Deployments
<br />

This section describes the scripts that can be run to deploy multiple contracts in a sequence.

<br />


### L1MessageService Chained Deployments
<br />

This will run the script that deploys PlonkVerifier, LineaRollup , Timelock contracts.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| SAVE_ADDRESS       | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash] |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH   | true      | bytes | Initial State Root Hash |
| LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER   | true      | uint256 | Initial L2 Block Number |
| LINEA_ROLLUP_SECURITY_COUNCIL  | true      | address | Security Council Address |
| LINEA_ROLLUP_OPERATORS     | true      | address | Operators Addresses (comma-delimited if multiple) |
| LINEA_ROLLUP_RATE_LIMIT_PERIOD     | true  | uint256   | L1 Rate Limit Period |
| LINEA_ROLLUP_RATE_LIMIT_AMOUNT     | true  | uint256   | L1 Rate Limit Amount |
| TIMELOCK_PROPOSERS | true     | address | Timelock Proposers address |
| TIMELOCK_EXECUTORS | true     | address | Timelock Executors address |
| TIMELOCK_ADMIN_ADDRESS | true     | address | Timelock Admin address |
| MIN_DELAY | true      | uint256 | Timelock Minimum Delay |
| PLONKVERIFIER_NAME | true | string | PlonkVerifier contract name that should be deployed |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags PlonkVerifier,LineaRollup,Timelock
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true SEPOLIA_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> LINEA_ROLLUP_INITIAL_STATE_ROOT_HASH=<bytes> LINEA_ROLLUP_INITIAL_L2_BLOCK_NUMBER=<value> LINEA_ROLLUP_SECURITY_COUNCIL=<address> LINEA_ROLLUP_OPERATORS=<address> LINEA_ROLLUP_RATE_LIMIT_PERIOD=<value> LINEA_ROLLUP_RATE_LIMIT_AMOUNT=<value> TIMELOCK_PROPOSERS=<address> TIMELOCK_EXECUTORS=<address> TIMELOCK_ADMIN_ADDRESS=<address> MIN_DELAY=<value> PLONKVERIFIER_NAME=PlonkVerifierForMultiTypeDataAggregation npx hardhat deploy --network sepolia --tags PlonkVerifier,LineaRollup,Timelock
```

(make sure to replace `<value>` `<bytes>` `<key>` `<address>` with actual values)

<br />
<br />

### L2MessageService Chained Deployments
<br />

This will run the script that deploys Timelock, L2MessageService contracts.

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| SAVE_ADDRESS       | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash] |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2MSGSERVICE_SECURITY_COUNCIL | true   | address | L2 Security council address |
| L2MSGSERVICE_L1L2_MESSAGE_SETTER  | true  |  address | L1L2 Message Setter address on L2 |
| L2MSGSERVICE_RATE_LIMIT_PERIOD    | true  |  uint256 | L2 Rate Limit Period |
| L2MSGSERVICE_RATE_LIMIT_AMOUNT    | true  |  uint256 | L2 Rate Limit Amount |
| TIMELOCK_PROPOSERS | true     | address | Timelock Proposers address |
| TIMELOCK_EXECUTORS | true     | address | Timelock Executors address |
| TIMELOCK_ADMIN_ADDRESS | true     | address | Timelock Admin address |
| MIN_DELAY | true      | uint256 | Timelock Minimum Delay |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags L2MessageService,Timelock
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true LINEA_SEPOLIA_PRIVATE_KEY=<key> LINEASCAN_API_KEY=<key> INFURA_API_KEY=<key> L2MSGSERVICE_SECURITY_COUNCIL=<address> L2MSGSERVICE_L1L2_MESSAGE_SETTER=<address>  L2MSGSERVICE_RATE_LIMIT_PERIOD=<value> L2MSGSERVICE_RATE_LIMIT_AMOUNT=<value> TIMELOCK_PROPOSERS=<address> TIMELOCK_EXECUTORS=<address> TIMELOCK_ADMIN_ADDRESS=<address> MIN_DELAY=<value> npx hardhat deploy --network linea_sepolia --tags L2MessageService_Timelock
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />
<br />

### TokenBridge & BridgedToken Chained Deployments

This will run the script that deploys the TokenBridge and BridgedToken contracts.

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| SAVE_ADDRESS          | false    |true\|false| Saves file with deployment details [address, ABI, transaction hash]. |
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contract. |
| \**PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2_MESSAGE_SERVICE_ADDRESS    | true  | address   | L2 Message Service address used when deploying TokenBridge.    |
| LINEA_ROLLUP_ADDRESS         | true    | address       | L1 Rollup address used when deploying Token Bridge.   |
| REMOTE_CHAIN_ID       | true      |   uint256     | ChainID of the remote (target) network |
| TOKEN_BRIDGE_L1       | false     |true\|false| If Token Bridge is deployed on L1, TOKEN_BRIDGE_L1 should be set to `true`. Otherwise it should be `false`|
| L1_RESERVED_TOKEN_ADDRESSES | false   | address   | If TOKEN_BRIDGE_L1=true, then L1_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|
| L2_RESERVED_TOKEN_ADDRESSES | false   | address   | If TOKEN_BRIDGE_L1=false, then L2_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|


Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags BridgedToken,TokenBridge
```

Base command with cli arguments:
```shell
SAVE_ADDRESS=true VERIFY_CONTRACT=true LINEASCAN_API_KEY=<key> LINEA_SEPOLIA_PRIVATE_KEY=<key> INFURA_API_KEY=<key> REMOTE_CHAIN_ID=<uint256> TOKEN_BRIDGE_L1=true L1_RESERVED_TOKEN_ADDRESSES=<address>  L2_MESSAGE_SERVICE_ADDRESS=<address> LINEA_ROLLUP_ADDRESS=<address>  npx hardhat deploy --network linea_sepolia --tags BridgedToken,TokenBridge
```
(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />
<br />