# @consensys/linea-sdk-core

## 1.0.0 (2026-04-10)

Initial release of `@consensys/linea-sdk-core`, the framework-agnostic foundation for Linea bridge SDKs.

### Features

- **Sparse Merkle Tree** -- `SparseMerkleTree` class with configurable depth and hash function; leaf management, root computation, and Merkle proof generation returning `MessageProof`
- **Block extra data parsing** -- `parseBlockExtraData` to decode Linea block `extraData` into version and fee parameters
- **Message status formatting** -- `formatMessageStatus` mapping on-chain numeric statuses to `OnChainMessageStatus` enum (`UNKNOWN`, `CLAIMABLE`, `CLAIMED`)
- **Contract address resolution** -- `getContractsAddressesByChainId` returning canonical addresses (Linea Rollup, Message Service, Token Bridge) for Ethereum Mainnet, Sepolia, Linea Mainnet, and Linea Sepolia
- **Chain identification** -- `isMainnet`, `isSepolia`, `isLineaMainnet`, `isLineaSepolia` predicates

### Types

Bridge client surface:

- `L1PublicClient` / `L2PublicClient` -- read-only bridge operations (message lookup, status, proof retrieval, block extra data)
- `L1WalletClient` / `L2WalletClient` -- write bridge operations (deposit, withdraw, claim)
- `Message`, `ExtendedMessage`, `MessageProof`, `OnChainMessageStatus`
- Framework-agnostic transaction types (logs, receipts, EIP-compliant request variants)
