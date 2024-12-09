# Linea Bridge SDK

## Description
The Linea SDK package a TypeScript library for seamless bridging operations between Ethereum (L1) and Linea (L2) networks. It provides functionality for interacting with contracts and retrieving message status and information.

## Features

- 🌉 **Bridge Operations**
  - Claim bridged assets on L1 and L2
  - Get message proof to claim on L1
  - Track message status across chains
  
- 🔍 **Message Tracking**
  - Query message status
  - Monitor bridge events
  
- ⚡ **Gas Management**
  - Automatic gas estimation
  - Custom fee strategies

## Installation

```bash
npm install @consensys/linea-sdk
# or
yarn add @consensys/linea-sdk
# or
pnpm add @consensys/linea-sdk
```

## Quick Start

```typescript
import { LineaSDK } from '@consensys/linea-sdk';

// Initialize SDK
const sdk = new LineaSDK({
  network: 'linea-mainnet', // or 'linea-sepolia' or 'custom'
  mode: 'read-write',
  l1RpcUrlOrProvider: 'YOUR_L1_RPC_URL',
  l2RpcUrlOrProvider: 'YOUR_L2_RPC_URL',
  l1SignerPrivateKeyOrWallet: 'YOUR_L1_PRIVATE_KEY',
  l2SignerPrivateKeyOrWallet: 'YOUR_L2_PRIVATE_KEY'
});

// Get L1 and L2 contract instances
const l1Contract = sdk.getL1Contract();
const l2Contract = sdk.getL2Contract();
```

## SDK Configuration

### LineaSDKOptions

The SDK can be initialized in two modes: `read-only` or `read-write`. The configuration options differ based on the mode:

#### Read-Only Mode
```typescript
interface ReadOnlyModeOptions {
  network: "linea-mainnet" | "linea-sepolia" | "custom";
  mode: "read-only";
  l1RpcUrlOrProvider: string | Eip1193Provider;
  l2RpcUrlOrProvider: string | Eip1193Provider;
  l2MessageTreeDepth?: number;
  l1FeeEstimatorOptions?: {
    maxFeePerGasCap?: bigint;
    gasFeeEstimationPercentile?: number;
    enforceMaxGasFee?: boolean;
  };
  l2FeeEstimatorOptions?: {
    maxFeePerGasCap?: bigint;
    gasFeeEstimationPercentile?: number;
    enforceMaxGasFee?: boolean;
  };
}
```

#### Read-Write Mode
```typescript
interface WriteModeOptions extends ReadOnlyModeOptions {
  mode: "read-write";
  l1SignerPrivateKeyOrWallet: string | Wallet;
  l2SignerPrivateKeyOrWallet: string | Wallet;
}
```

#### Field Explanations
- `network`: `"linea-mainnet"` | `"linea-sepolia"` | `"custom"`
  - Description: Specifies the blockchain network to connect to.
  - Possible Values:
    - `"linea-mainnet"`: Connects to the Linea Mainnet contracts.
    - `"linea-sepolia"`: Connects to the Linea Sepolia testnet contracts.
    - `"custom"`: Allows for a custom contracts addresses, requiring custom RPC URLs for L1 and L2.
- `mode`: `"read-only"` | `"read-write"`
  - Description: Determines the operation mode of the client.
  - Possible Values:
    - `"read-only"`: The client operates without the ability to send transactions; it can only read data from the blockchain.
    - `"read-write"`: The client can read data and also send transactions, requiring signing credentials.
- `l1RpcUrlOrProvider`: `string | Eip1193Provider`
  - Description: The RPC URL or provider for connecting to Layer 1 (L1) of the blockchain.
  - Options:
    - `string`: A URL pointing to the L1 RPC endpoint.
    - `Eip1193Provider`: An EIP-1193 compliant provider instance.
- `l2RpcUrlOrProvider`: `string | Eip1193Provider`
  - Description: The RPC URL or provider for connecting to Layer 2 (L2) of the blockchain.
  - Options:
    - `string`: A URL pointing to the L2 RPC endpoint.
    - `Eip1193Provider`: An EIP-1193 compliant provider instance.
- `l2MessageTreeDepth?`: `number` (Optional)
  - Description: Specifies the depth of the L2 message tree used in cryptographic operations or data structures.
  - Default: If not provided, a default value of `5` is used which is the value used in Mainnet and Sepolia.
- `l1FeeEstimatorOptions?`: (Optional)
  - Description: Configuration options for estimating transaction fees on Layer 1.
  - Fields:
    - `maxFeePerGasCap?`: `bigint` (Optional)
      - Description: The maximum gas price (in wei) you're willing to pay per unit of gas on L1.
      - Default: If not provided, a default value of `100000000000n` is used.
    - `gasFeeEstimationPercentile?`: `number` (Optional)
      - Description: The percentile of recent gas prices to use for fee estimation (used in `eth_feeHistory`).
      - Default: If not provided, a default value of `20` is used.
    - `enforceMaxGasFee?`: `boolean` (Optional)
      - Description: If true, ensures the gas fee does not exceed maxFeePerGasCap.
      - Default: `false`
- `l2FeeEstimatorOptions?`: (Optional)
  - Description: Configuration options for estimating transaction fees on Layer 2.
  - Fields:
    - `maxFeePerGasCap?`: `bigint` (Optional)
      - Description: The maximum gas price (in wei) you're willing to pay per unit of gas on L2.
      - Default: If not provided, a default value of `100000000000n` is used.
    - `gasFeeEstimationPercentile?`: `number` (Optional)
      - Description: The percentile of recent gas prices to use for fee estimation (used in `eth_feeHistory`).
      - Default: If not provided, a default value of `20` is used.
    - `enforceMaxGasFee?`: `boolean` (Optional)
      - Description: If true, ensures the gas fee does not exceed maxFeePerGasCap.
      - Default: `false`
- `l1SignerPrivateKeyOrWallet`: `string | Wallet` <strong>(Required in "read-write" mode)</strong>
  - Description: Credentials used to sign transactions on Layer 1.
  - Options:
    - `string`: A hexadecimal string representing the private key.
    - `Wallet`: A wallet instance (e.g., from ethers.js) containing the private key and signing functionality.
- `l2SignerPrivateKeyOrWallet`: `string | Wallet` <strong>(Required in "read-write" mode)</strong>
  - Description: Credentials used to sign transactions on Layer 2.
  - Options:
    - `string`: A hexadecimal string representing the private key.
    - `Wallet`: A wallet instance for signing L2 transactions.

#### Additional Notes

- <strong>Common Fields</strong>: The fields `network`, `mode`, `l1RpcUrlOrProvider`, `l2RpcUrlOrProvider`, `l2MessageTreeDepth`, `l1FeeEstimatorOptions`, and `l2FeeEstimatorOptions` are common to both `read-only` and `read-write` modes.
- <strong>Mode-Specific Fields</strong>:
  - In `read-only` mode:
    - Only the common fields are required.
    - The client can interact with the blockchain to read data but cannot send transactions.
  - In `read-write` mode:
    - All common fields are required.
    - Additional Required Fields: l1SignerPrivateKeyOrWallet and l2SignerPrivateKeyOrWallet are necessary to enable transaction signing and sending capabilities.
- <strong>Fee Estimator Options</strong>:
  - `maxFeePerGasCap`: Sets an upper limit on the gas price you're willing to pay.
  - `gasFeeEstimationPercentile`: Helps in choosing a gas price based on recent network activity (used in `eth_feeHistory`).
  - `enforceMaxGasFee`: Ensures that the gas fee does not exceed the maxFeePerGasCap value, providing cost control.

#### Usage Summary

- To <strong>read data only</strong> from the blockchain:
  - Set `mode` to `read-only`.
  - Provide the necessary network and RPC configurations.
- To <strong>read and write</strong> data (send transactions):
  - Set `mode` to `read-write`.
  - Provide all the common fields plus the signing credentials (`l1SignerPrivateKeyOrWallet` and `l2SignerPrivateKeyOrWallet`).

## Main Components

### L1 Contract (LineaRollupClient)

The L1 contract handles operations on the Ethereum side.

#### Message Operations
```typescript
// Get message by hash
const message = await l1Contract.getMessageByMessageHash(messageHash);

// Get messages by transaction hash
const messages = await l1Contract.getMessagesByTransactionHash(txHash);

// Get message status
const status = await l1Contract.getMessageStatus(messageHash);

// Get transaction receipt by message hash
const receipt = await l1Contract.getTransactionReceiptByMessageHash(messageHash);
```

#### Claiming Operations
```typescript
// Estimate claim gas
const gas = await l1Contract.estimateClaimGas(message);

// Claim message
const tx = await l1Contract.claim(message);
```

### L2 Contract (L2MessageServiceClient)

The L2 contract handles operations on the Linea side.

#### Message Operations
```typescript
// Get message by hash
const message = await l2Contract.getMessageByMessageHash(messageHash);

// Get messages by transaction hash
const messages = await l2Contract.getMessagesByTransactionHash(txHash);

// Get message status
const status = await l2Contract.getMessageStatus(messageHash);

// Get transaction receipt by message hash
const receipt = await l2Contract.getTransactionReceiptByMessageHash(messageHash);
```

#### Claiming and Transaction Operations
```typescript

// Estimate claim gas and fees
const gasFees = await l2Contract.estimateClaimGasFees(message)

// Claim message on L2
const tx = await l2Contract.claim(message);

```

## License

This package is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
