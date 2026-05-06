# @consensys/linea-sdk-viem

## 1.0.0 (2026-04-10)

Initial release of `@consensys/linea-sdk-viem`, a TypeScript SDK for interacting with the Linea bridge and messaging system built on [Viem](https://viem.sh/).

### Features

- **ETH and ERC-20 deposits** -- `deposit` action to bridge tokens or native ETH from L1 to L2 via the Linea Token Bridge
- **ETH and ERC-20 withdrawals** -- `withdraw` action to bridge tokens or native ETH from L2 to L1
- **L1 message claiming** -- `claimOnL1` action to finalize an L2-to-L1 message with a Merkle proof (auto-fetched or caller-supplied)
- **L2 message claiming** -- `claimOnL2` action to finalize an L1-to-L2 message on the L2 Message Service
- **L1-to-L2 message status** -- `getL1ToL2MessageStatus` to query whether an L1-to-L2 message is unknown, claimable, or claimed
- **L2-to-L1 message status** -- `getL2ToL1MessageStatus` to query whether an L2-to-L1 message is unknown, claimable, or claimed, including finalization checks
- **Message proof retrieval** -- `getMessageProof` to fetch the Merkle proof required for claiming an L2-to-L1 message on L1
- **Message lookup by hash** -- `getMessageByMessageHash` to retrieve full message details from on-chain `MessageSent` events
- **Message lookup by transaction** -- `getMessagesByTransactionHash` to retrieve all messages emitted in a given transaction
- **Receipt lookup by message hash** -- `getTransactionReceiptByMessageHash` to find the transaction receipt that contains a specific message
- **MessageSent event queries** -- `getMessageSentEvents` to fetch `MessageSent` logs for a given block range
- **Block extra data parsing** -- `getBlockExtraData` to decode Linea-specific block `extraData` into version and fee parameters
- **Message hash computation** -- `computeMessageHash` utility to deterministically hash message parameters

### Decorators

- **`publicActionsL1` / `publicActionsL2`** -- extend a Viem public client with all Linea read actions (message status, proofs, events, block extra data)
- **`walletActionsL1` / `walletActionsL2`** -- extend a Viem wallet client with all Linea write actions (deposit, withdraw, claim)
- All decorators accept optional custom contract addresses for non-standard deployments

### Network Support

- Ethereum Mainnet, Linea Mainnet, Sepolia, and Linea Sepolia with automatic contract address resolution via `@consensys/linea-sdk-core`

### Error Handling

- Structured Viem `BaseError` subclasses: `MessageNotFoundError`, `L2BlockNotFinalizedError`, `MessagesNotFoundInBlockRangeError`, `MerkleRootNotFoundInFinalizationDataError`, `EventNotFoundInFinalizationDataError`, `MissingMessageProofOrClientForClaimingOnL1Error`, `AccountNotFoundError`


