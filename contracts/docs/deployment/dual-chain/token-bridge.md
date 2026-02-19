# TokenBridge

[← Back to index](../README.md)

<br />

The TokenBridge can be deployed on either L1 or L2, controlled by the `TOKEN_BRIDGE_L1` flag. The BridgedToken beacon must be deployed first, as TokenBridge references it during initialization.

<br />

## BridgedToken

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contract. |
| \**DEPLOYER_PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags BridgedToken
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true ETHERSCAN_API_KEY=<key> DEPLOYER_PRIVATE_KEY=<key> INFURA_API_KEY=<key> npx hardhat deploy --network linea_sepolia --tags BridgedToken
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />

## CustomBridgedToken

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| CUSTOMTOKENBRIDGE_NAME | true    |string| Token's name |
| CUSTOMTOKENBRIDGE_SYMBOL | true    |string| Token's symbol |
| CUSTOMTOKENBRIDGE_DECIMALS | true    |uint256| Token's decimals |
| CUSTOMTOKENBRIDGE_BRIDGE_ADDRESS | true    |address| Token bridge's address|
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contract. |
| \**DEPLOYER_PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags CustomBridgedToken
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true ETHERSCAN_API_KEY=<key> DEPLOYER_PRIVATE_KEY=<key> INFURA_API_KEY=<key> CUSTOMTOKENBRIDGE_NAME=<name> CUSTOMTOKENBRIDGE_SYMBOL=<symbol> CUSTOMTOKENBRIDGE_DECIMALS=<decimals> CUSTOMTOKENBRIDGE_BRIDGE_ADDRESS=<address> npx hardhat deploy --network linea_sepolia --tags CustomBridgedToken
```

(make sure to replace `<key>` `<address>` `<name>` `<symbol>` `<decimals>` with actual values)

<br />

## TokenBridge

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contract. |
| \**DEPLOYER_PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2_MESSAGE_SERVICE_ADDRESS    | true  | address   | L2 Message Service address used when deploying TokenBridge.    |
| LINEA_ROLLUP_ADDRESS         | true    | address       | L1 Rollup address used when deploying Token Bridge.   |
| REMOTE_CHAIN_ID       | true      |   uint256     | ChainID of the remote (target) network |
| REMOTE_SENDER_ADDRESS | true | address | Remote sender address (the TokenBridge on the other chain) |
| BRIDGED_TOKEN_ADDRESS | true | address | BridgedToken beacon address (deploy BridgedToken first) |
| TOKEN_BRIDGE_L1       | false     |true\|false| If Token Bridge is deployed on L1, TOKEN_BRIDGE_L1 should be set to `true`. Otherwise it should be `false`|
| L1_SECURITY_COUNCIL | conditional | address | L1 Security Council address. Required when `TOKEN_BRIDGE_L1=true` |
| L2_SECURITY_COUNCIL | conditional | address | L2 Security Council address. Required when `TOKEN_BRIDGE_L1=false` |
| L1_RESERVED_TOKEN_ADDRESSES | false   | address   | If TOKEN_BRIDGE_L1=true, then L1_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|
| L2_RESERVED_TOKEN_ADDRESSES | false   | address   | If TOKEN_BRIDGE_L1=false, then L2_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags TokenBridge
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true ETHERSCAN_API_KEY=<key> DEPLOYER_PRIVATE_KEY=<key> INFURA_API_KEY=<key> REMOTE_CHAIN_ID=<uint256> TOKEN_BRIDGE_L1=true L1_SECURITY_COUNCIL=<address> L1_RESERVED_TOKEN_ADDRESSES=<address> L2_MESSAGE_SERVICE_ADDRESS=<address> LINEA_ROLLUP_ADDRESS=<address> REMOTE_SENDER_ADDRESS=<address> BRIDGED_TOKEN_ADDRESS=<address> npx hardhat deploy --network linea_sepolia --tags TokenBridge
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />

## Upgrade Deployments

### TokenBridgeWithReinitialization

Deploys a new TokenBridge implementation and generates encoded upgrade calldata with `reinitializeV2`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| TOKEN_BRIDGE_ADDRESS | true | address | Existing TokenBridge proxy address |

```shell
npx hardhat deploy --network sepolia --tags TokenBridgeWithReinitialization
```
