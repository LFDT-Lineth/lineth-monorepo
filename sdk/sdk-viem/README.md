# @consensys/linea-sdk-viem

A TypeScript SDK for interacting with the Linea bridge and messaging system, built on top of [Viem](https://viem.sh/). This package provides high-level actions and decorators for bridging tokens, sending/claiming messages, and querying message status between L1 and L2 on Linea.

> **Note:** This SDK supports both mainnet and testnet environments, including Ethereum Mainnet, Linea Mainnet, Sepolia, and Linea Sepolia. Simply use the appropriate chain from `viem/chains` (e.g., `mainnet`, `linea`, `sepolia`, `lineaSepolia`).

## Installation

```bash
npm install @consensys/linea-sdk-viem viem
# or
yarn add @consensys/linea-sdk-viem viem
# or
pnpm add @consensys/linea-sdk-viem viem
```

> **Note:** `viem@>=2.22.0` is a required peer dependency.

## Usage

Import the functions or decorators you need, and use the appropriate chain for your network (mainnet or testnet):

```ts
import { deposit, withdraw, getBlockExtraData, publicActionsL1, walletActionsL2 } from '@consensys/linea-sdk-viem';
import { mainnet, linea, sepolia, lineaSepolia } from 'viem/chains';

// Example: Use sepolia and lineaSepolia for testnet
const l1Client = createPublicClient({ chain: sepolia, transport: http() });
const l2Client = createPublicClient({ chain: lineaSepolia, transport: http() });
```

---

## Wallet Actions

These functions require a Viem wallet client (e.g., `createWalletClient`). They are used for sending transactions that modify state, such as deposits, withdrawals, and claiming messages.

### deposit
Deposits tokens from L1 to L2 or ETH if `token` is set to `zeroAddress`.

```ts
import { createWalletClient, http, zeroAddress } from 'viem'
import { privateKeyToAccount } from 'viem/accounts'
import { mainnet, linea, sepolia, lineaSepolia } from 'viem/chains'
import { deposit } from '@consensys/linea-sdk-viem'

// Use mainnet/linea for mainnet, sepolia/lineaSepolia for testnet
const client = createWalletClient({
  chain: sepolia,
  transport: http(),
});
const l2Client = createPublicClient({
  chain: lineaSepolia,
  transport: http(),
});
const hash = await deposit(client, {
  l2Client,
  account: privateKeyToAccount('0x…'),
  amount: 1_000_000_000_000n,
  token: zeroAddress, // Use zeroAddress for ETH
  to: '0xRecipientAddress',
  data: '0x', // Optional data
  fee: 100_000_000n, // Optional fee
});
```

### withdraw
Withdraws tokens from L2 to L1 or ETH if `token` is set to `zeroAddress`.

```ts
import { createWalletClient, http, zeroAddress } from 'viem'
import { privateKeyToAccount } from 'viem/accounts'
import { linea, lineaSepolia } from 'viem/chains'
import { withdraw } from '@consensys/linea-sdk-viem'

// Use linea for mainnet, lineaSepolia for testnet
const client = createWalletClient({
  chain: lineaSepolia,
  transport: http(),
});
const hash = await withdraw(client, {
  account: privateKeyToAccount('0x…'),
  amount: 1_000_000_000_000n,
  token: zeroAddress, // Use zeroAddress for ETH
  to: '0xRecipientAddress',
  data: '0x', // Optional data
});
```

### claimOnL1
Claim a message on L1.

```ts
import { createWalletClient, http, zeroAddress } from 'viem'
import { privateKeyToAccount } from 'viem/accounts'
import { mainnet, sepolia } from 'viem/chains'
import { claimOnL1 } from '@consensys/linea-sdk-viem'

// Use mainnet for mainnet, sepolia for testnet
const client = createWalletClient({
  chain: sepolia,
  transport: http(),
});
const hash = await claimOnL1(client, {
  account: privateKeyToAccount('0x…'),
  from: '0xSenderAddress',
  to: '0xRecipientAddress',
  fee: 100_000_000n,
  value: 1_000_000_000_000n,
  messageNonce: 1n,
  calldata: '0x',
  feeRecipient: zeroAddress,
  messageProof: {
    root: '0x…',
    proof: ['0x…'],
    leafIndex: 0,
  },
});
```

### claimOnL2
Claim a message on L2.

```ts
import { createWalletClient, http, zeroAddress } from 'viem'
import { privateKeyToAccount } from 'viem/accounts'
import { linea, lineaSepolia } from 'viem/chains'
import { claimOnL2 } from '@consensys/linea-sdk-viem'

// Use linea for mainnet, lineaSepolia for testnet
const client = createWalletClient({
  chain: lineaSepolia,
  transport: http(),
});
const hash = await claimOnL2(client, {
  account: privateKeyToAccount('0x…'),
  from: '0xSenderAddress',
  to: '0xRecipientAddress',
  fee: 100_000_000n,
  value: 1_000_000_000_000n,
  messageNonce: 1n,
  calldata: '0x',
  feeRecipient: zeroAddress,
  // Optional transaction parameters
  gas: 21000n, // Gas limit
  maxFeePerGas: 100_000_000n, // Max fee per gas
  maxPriorityFeePerGas: 1_000_000n, // Max priority fee per gas
});
```

---

## Public Actions

These functions can be called on a Viem public client (e.g., `createPublicClient`). They are used for reading data from the blockchain, such as querying message status, proofs, and events. All actions support both mainnet and testnet chains (mainnet/linea, sepolia/lineaSepolia).

### getBlockExtraData
Returns formatted Linea block extra data.

```ts
import { createPublicClient, http } from 'viem'
import { linea, lineaSepolia } from 'viem/chains'
import { getBlockExtraData } from '@consensys/linea-sdk-viem'

// Use linea for mainnet, lineaSepolia for testnet
const client = createPublicClient({
  chain: lineaSepolia,
  transport: http(),
});
const blockExtraData = await getBlockExtraData(client, {
  blockTag: 'latest',
});
```

### getL1ToL2MessageStatus
Returns the status of an L1 to L2 message on Linea.

```ts
import { createPublicClient, http } from 'viem'
import { linea, lineaSepolia } from 'viem/chains'
import { getL1ToL2MessageStatus } from '@consensys/linea-sdk-viem'

const client = createPublicClient({
  chain: lineaSepolia,
  transport: http(),
});
const messageStatus = await getL1ToL2MessageStatus(client, {
  messageHash: '0x1234…',
});
```

### getL2ToL1MessageStatus
Returns the status of an L2 to L1 message on Linea.

```ts
import { createPublicClient, http } from 'viem'
import { mainnet, linea, sepolia, lineaSepolia } from 'viem/chains'
import { getL2ToL1MessageStatus } from '@consensys/linea-sdk-viem'

const client = createPublicClient({
  chain: sepolia,
  transport: http(),
});
const l2Client = createPublicClient({
  chain: lineaSepolia,
  transport: http(),
});
const messageStatus = await getL2ToL1MessageStatus(client, {
  l2Client,
  messageHash: '0x1234…',
});
```

### getMessageByMessageHash
Returns the details of a message by its hash.

```ts
import { createPublicClient, http } from 'viem'
import { linea, lineaSepolia } from 'viem/chains'
import { getMessageByMessageHash } from '@consensys/linea-sdk-viem'

const client = createPublicClient({
  chain: lineaSepolia,
  transport: http(),
});
const message = await getMessageByMessageHash(client, {
  messageHash: '0x1234…',
});
```

### getMessageProof
Returns the proof of a message sent from L2 to L1.

```ts
import { createPublicClient, http } from 'viem'
import { mainnet, linea, sepolia, lineaSepolia } from 'viem/chains'
import { getMessageProof } from '@consensys/linea-sdk-viem'

const client = createPublicClient({
  chain: sepolia,
  transport: http(),
});
const l2Client = createPublicClient({
  chain: lineaSepolia,
  transport: http(),
});
const messageProof = await getMessageProof(client, {
  l2Client,
  messageHash: '0x1234…',
});
```

### getMessagesByTransactionHash
Returns the details of messages sent in a transaction by its hash.

```ts
import { createPublicClient, http } from 'viem'
import { linea, lineaSepolia } from 'viem/chains'
import { getMessagesByTransactionHash } from '@consensys/linea-sdk-viem'

const client = createPublicClient({
  chain: lineaSepolia,
  transport: http(),
});
const messages = await getMessagesByTransactionHash(client, {
  transactionHash: '0x1234…',
});
```

### getTransactionReceiptByMessageHash
Returns the transaction receipt for a message sent by its message hash.

```ts
import { createPublicClient, http } from 'viem'
import { linea, lineaSepolia } from 'viem/chains'
import { getTransactionReceiptByMessageHash } from '@consensys/linea-sdk-viem'

const client = createPublicClient({
  chain: lineaSepolia,
  transport: http(),
});
const receipt = await getTransactionReceiptByMessageHash(client, {
  messageHash: '0x1234…',
});
```

---

## Decorators

Decorators allow you to extend a Viem client (public or wallet) with additional Linea-specific actions. Each decorator can take optional parameters to specify custom contract addresses, which is useful for advanced or non-standard deployments. Decorators and all actions support both mainnet and testnet (Linea Mainnet, Ethereum Mainnet, Sepolia, and Linea Sepolia).

### publicActionsL1 / publicActionsL2

Extend a Viem public client with Linea public actions for L1 or L2. You can optionally pass an object with custom contract addresses:

- **publicActionsL1 parameters:**
  - `lineaRollupAddress` (Address): Custom Linea rollup contract address on L1
  - `l2MessageServiceAddress` (Address): Custom L2 message service contract address
- **publicActionsL2 parameters:**
  - `lineaRollupAddress` (Address): Custom Linea rollup contract address on L2
  - `l2MessageServiceAddress` (Address): Custom L2 message service contract address

**Default usage:**
```ts
import { createPublicClient, http } from 'viem';
import { mainnet, linea, sepolia, lineaSepolia } from 'viem/chains';
import { publicActionsL1, publicActionsL2 } from '@consensys/linea-sdk-viem';

const l1Client = createPublicClient({ chain: sepolia, transport: http() }).extend(publicActionsL1());
const l2Client = createPublicClient({ chain: lineaSepolia, transport: http() }).extend(publicActionsL2());
```

**With custom contract addresses:**
```ts
const l1Client = createPublicClient({ chain: sepolia, transport: http() }).extend(
  publicActionsL1({
    lineaRollupAddress: '0xYourCustomL1Rollup',
    l2MessageServiceAddress: '0xYourCustomL2MessageService',
  })
);
```

### walletActionsL1 / walletActionsL2

Extend a Viem wallet client with Linea wallet actions for L1 or L2. You can optionally pass an object with custom contract addresses:

- **walletActionsL1 parameters:**
  - `lineaRollupAddress` (Address): Custom Linea rollup contract address on L1
  - `l2MessageServiceAddress` (Address): Custom L2 message service contract address
  - `l1TokenBridgeAddress` (Address): Custom L1 token bridge contract address
  - `l2TokenBridgeAddress` (Address): Custom L2 token bridge contract address
- **walletActionsL2 parameters:**
  - `l2MessageServiceAddress` (Address): Custom L2 message service contract address
  - `l2TokenBridgeAddress` (Address): Custom L2 token bridge contract address

**Default usage:**
```ts
import { createWalletClient, http } from 'viem';
import { mainnet, linea, sepolia, lineaSepolia } from 'viem/chains';
import { walletActionsL1, walletActionsL2 } from '@consensys/linea-sdk-viem';

const l1Wallet = createWalletClient({ chain: sepolia, transport: http() }).extend(walletActionsL1());
const l2Wallet = createWalletClient({ chain: lineaSepolia, transport: http() }).extend(walletActionsL2());
```

**With custom contract addresses:**
```ts
const l1Wallet = createWalletClient({ chain: sepolia, transport: http() }).extend(
  walletActionsL1({
    lineaRollupAddress: '0xYourCustomL1Rollup',
    l2MessageServiceAddress: '0xYourCustomL2MessageService',
    l1TokenBridgeAddress: '0xYourCustomL1TokenBridge',
    l2TokenBridgeAddress: '0xYourCustomL2TokenBridge',
  })
);
```

---

## Contributing

1. **Install dependencies:**
   ```bash
   pnpm install
   ```
2. **Build:**
   ```bash
   pnpm run build
   ```
3. **Lint:**
   ```bash
   pnpm run lint:fix
   ```
4. **Test:**
   ```bash
   pnpm run test
   ```
