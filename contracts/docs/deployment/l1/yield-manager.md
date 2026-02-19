# YieldManager

[← Back to index](../README.md)

<br />

## YieldManager (Fresh Deploy)

Deploys YieldManager, ValidatorContainerProofVerifier and LidoStVaultYieldProviderFactory contracts.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| LINEA_ROLLUP_ADDRESS | true | address | LineaRollup contract address |
| L1_SECURITY_COUNCIL  | true      | address | L1 Security Council Address |
| NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS | true | address | Automation service address |
| VAULT_HUB | true | address | Lido Vault Hub address |
| VAULT_FACTORY | true | address | Lido Vault Factory address |
| STETH | true | address | stETH token address |
| MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS | true | uint256 | Minimum withdrawal reserve percentage in basis points |
| TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS | true | uint256 | Target withdrawal reserve percentage in basis points |
| MINIMUM_WITHDRAWAL_RESERVE_AMOUNT | true | uint256 | Minimum withdrawal reserve amount |
| TARGET_WITHDRAWAL_RESERVE_AMOUNT | true | uint256 | Target withdrawal reserve amount |
| GI_FIRST_VALIDATOR | false | bytes32 | Generalized index for first validator |
| GI_PENDING_PARTIAL_WITHDRAWALS_ROOT | false | bytes32 | Generalized index for pending partial withdrawals root |
| VALIDATOR_CONTAINER_PROOF_VERIFIER_ADMIN | false | address | Admin address for the ValidatorContainerProofVerifier |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags YieldManager
```

<br />

## YieldManagerArtifacts

Deploys YieldManager, ValidatorContainerProofVerifier and LidoStVaultYieldProviderFactory from audited artifacts. Use this instead of `YieldManager` when deploying from pre-built, audit-signed bytecode.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| LINEA_ROLLUP_ADDRESS | true | address | LineaRollup contract address |
| L1_SECURITY_COUNCIL  | true      | address | L1 Security Council Address |
| NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS | true | address | Automation service address |
| VAULT_HUB | true | address | Lido Vault Hub address |
| VAULT_FACTORY | true | address | Lido Vault Factory address |
| STETH | true | address | stETH token address |
| MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS | true | uint256 | Minimum withdrawal reserve percentage in basis points |
| TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS | true | uint256 | Target withdrawal reserve percentage in basis points |
| MINIMUM_WITHDRAWAL_RESERVE_AMOUNT | true | uint256 | Minimum withdrawal reserve amount |
| TARGET_WITHDRAWAL_RESERVE_AMOUNT | true | uint256 | Target withdrawal reserve amount |
| GI_FIRST_VALIDATOR | false | bytes32 | Generalized index for first validator |
| GI_PENDING_PARTIAL_WITHDRAWALS_ROOT | false | bytes32 | Generalized index for pending partial withdrawals root |
| VALIDATOR_CONTAINER_PROOF_VERIFIER_ADMIN | false | address | Admin address for the ValidatorContainerProofVerifier |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags YieldManagerArtifacts
```

<br />

## YieldManagerImplementation

Deploys a new YieldManager implementation contract (without proxy). Use this when upgrading an existing YieldManager proxy.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| LINEA_ROLLUP_ADDRESS | true | address | LineaRollup contract address (constructor argument) |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags YieldManagerImplementation
```
