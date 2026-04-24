# Verifier (PlonkVerifier)

[← Back to index](../README.md)

<br />

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name             | Required | Input Value | Description |
| -------------------------- | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| VERIFIER_CONTRACT_NAME | true  | string | The name of the PlonkVerifier contract that should be deployed |
| VERIFIER_PROOF_TYPE | true  | string | The proof type that the verifier should be mapped to |
| VERIFIER_CHAIN_ID | true  | uint256 | Chain ID passed to the verifier constructor |
| VERIFIER_BASE_FEE | true  | uint256 | Base fee passed to the verifier constructor |
| VERIFIER_COINBASE | true  | address | Coinbase address passed to the verifier constructor |
| L2_MESSAGE_SERVICE_ADDRESS | true  | address | L2 Message Service address passed to the verifier constructor |
| VERIFIER_MIMC_ADDRESS | false | address | When set, links the Plonk verifier against this **already deployed** `Mimc` library instead of deploying a new one. Use this to reuse a shared `Mimc` deployment across multiple verifier upgrades or environments. Must be a checksummed or valid hex address with bytecode on the target network. When unset, the script deploys `Mimc` and verifies it when `VERIFY_CONTRACT=true`. |

<br />

Base command:
```shell
pnpm exec hardhat deploy --network sepolia --tags PlonkVerifier
```

Base command with cli arguments:

```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> VERIFIER_CONTRACT_NAME=PlonkVerifierDev pnpm exec hardhat deploy --network sepolia --tags PlonkVerifier
```

Reuse an existing `Mimc` library (no new `Mimc` deployment or `Mimc` explorer verification from this script):

```shell
VERIFIER_MIMC_ADDRESS=<0x-address-of-deployed-Mimc-library> npx hardhat deploy --network sepolia --tags PlonkVerifier
```

(make sure to replace `<key>` and `<0x-address-of-deployed-Mimc-library>` with actual values)
