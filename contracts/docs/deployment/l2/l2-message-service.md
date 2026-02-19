# L2MessageService

[← Back to index](../README.md)

<br />

## L2MessageService (Fresh Deploy)

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2_SECURITY_COUNCIL | true   | address | L2 Security council address |
| L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER  | true  |  address | L1L2 Message Setter address on L2 |
| L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD    | true  |  uint256 | L2 Rate Limit Period |
| L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT    | true  |  uint256 | L2 Rate Limit Amount |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags L2MessageService
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> L2_SECURITY_COUNCIL=<address> L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER=<address> L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD=<value> L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT=<value> npx hardhat deploy --network linea_sepolia --tags L2MessageService
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />

## L2MessageServiceLineaMainnet

Deploys L2MessageService using the V1-deployed ABI and bytecode (for Linea mainnet upgrade compatibility). Uses the same env vars as L2MessageService.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| L2_SECURITY_COUNCIL | true | address | L2 Security council address |
| L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER | true | address | L1L2 Message Setter address on L2 |
| L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD | true | uint256 | L2 Rate Limit Period |
| L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT | true | uint256 | L2 Rate Limit Amount |

<br />

Base command:
```shell
npx hardhat deploy --network linea_mainnet --tags L2MessageServiceLineaMainnet
```

<br />

## Upgrade Deployments

### L2MessageServiceWithReinitialization

Deploys a new L2MessageService implementation and generates encoded upgrade calldata with `reinitializeV3`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| L2_MESSAGE_SERVICE_ADDRESS | true | address | Existing L2MessageService proxy address |

```shell
npx hardhat deploy --network linea_sepolia --tags L2MessageServiceWithReinitialization
```
