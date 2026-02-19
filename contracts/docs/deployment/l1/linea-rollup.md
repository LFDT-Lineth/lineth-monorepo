# LineaRollup

[← Back to index](../README.md)

<br />

## LineaRollup (Fresh Deploy)

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network.|
| INITIAL_L2_STATE_ROOT_HASH   | true      | bytes | Initial State Root Hash (shared base) |
| INITIAL_L2_BLOCK_NUMBER   | true      | uint256 | Initial L2 Block Number (shared base) |
| L2_GENESIS_TIMESTAMP | true | uint256 | Genesis timestamp (shared base) |
| L1_SECURITY_COUNCIL  | true      | address | L1 Security Council Address |
| LINEA_ROLLUP_OPERATORS     | true      | address | L1 Operators Addresses (comma-delimited if multiple) |
| LINEA_ROLLUP_RATE_LIMIT_PERIOD     | true  | uint256   | L1 Rate Limit Period |
| LINEA_ROLLUP_RATE_LIMIT_AMOUNT     | true  | uint256   | L1 Rate Limit Amount |
| PLONKVERIFIER_ADDRESS | true | address | PlonkVerifier contract address (set automatically when deploying Verifier in same chain) |
| YIELD_MANAGER_ADDRESS | true | address | Yield Manager contract address |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags LineaRollup
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> INITIAL_L2_STATE_ROOT_HASH=<bytes> INITIAL_L2_BLOCK_NUMBER=<value> L2_GENESIS_TIMESTAMP=<value> L1_SECURITY_COUNCIL=<address> LINEA_ROLLUP_OPERATORS=<address> LINEA_ROLLUP_RATE_LIMIT_PERIOD=<value> LINEA_ROLLUP_RATE_LIMIT_AMOUNT=<value> YIELD_MANAGER_ADDRESS=<address> npx hardhat deploy --network sepolia --tags LineaRollup
```

(make sure to replace `<value>` `<key>` `<bytes>` `<address>` with actual values).

<br />

## Upgrade Deployments

### LineaRollupWithReinitialization

Deploys a new LineaRollup implementation and generates encoded upgrade calldata with `reinitializeV8`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| L1_SECURITY_COUNCIL | true | address | Security Council address |
| LINEA_ROLLUP_ADDRESS | true | address | Existing LineaRollup proxy address |

```shell
npx hardhat deploy --network sepolia --tags LineaRollupWithReinitialization
```

<br />

### LineaRollupV8WithReinitialization

Deploys LineaRollup from audited artifacts and generates encoded upgrade calldata with `reinitializeV8`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| L1_SECURITY_COUNCIL | true | address | Security Council address |
| LINEA_ROLLUP_ADDRESS | true | address | Existing LineaRollup proxy address |
| NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS | true | address | Automation service address |

```shell
npx hardhat deploy --network sepolia --tags LineaRollupV8WithReinitialization
```
