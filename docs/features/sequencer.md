# Sequencer

> Linea's block-producing Besu plugin with transaction selection, validation, and gas estimation.

## Overview

The sequencer is a Linea Besu node with QBFT consensus (via the [Maru](https://github.com/Consensys/maru) consensus client) that serves as the block producer. It uses a plugin architecture (`besu-plugins/linea-sequencer/`) to enforce Linea-specific transaction selection and validation rules beyond standard Ethereum mempool logic. The sequencer is not directly accessible externally; RPC nodes forward transactions to it via P2P.

Blocks are only produced when transactions exist — no empty blocks are created.

## Components

| Component | Path | Role |
|-----------|------|------|
| LineaTransactionSelectorPlugin | `besu-plugins/linea-sequencer/sequencer/` | Registers transaction selectors |
| LineaTransactionValidatorPlugin | `besu-plugins/linea-sequencer/sequencer/` | Registers pool validators |
| LineaTransactionSelector | `sequencer/.../selectors/` | Composite selector orchestrating all rules |
| LineaTransactionPoolValidatorFactory | `sequencer/.../txpoolvalidation/` | Chains all pool validators |

## Transaction Selection (Block Building)

When building a block, the sequencer applies these selectors in order:

| Selector | Rule |
|----------|------|
| `TraceLineLimitTransactionSelector` | Trace counts must fit within prover capacity |
| `AllowedAddressTransactionSelector` | Sender/recipient not on deny list |
| `MaxBlockCallDataTransactionSelector` | Block calldata size below configured limit |
| `MaxBlockGasTransactionSelector` | Block gas below `gasLimit` |
| `ProfitableTransactionSelector` | Gas fee / compressed size ratio is profitable |
| `BundleConstraintTransactionSelector` | Bundle atomicity constraints |
| `MaxBundleGasPerBlockTransactionSelector` | Bundle gas cap per block |
| `TransactionEventSelector` | Event-based selection logic |

Transactions exceeding trace limits are moved to an in-memory unexecutable list and removed from the pool.

## Transaction Pool Validation

Before entering the mempool, transactions pass through:

| Validator | Rule |
|-----------|------|
| `TraceLineLimitValidator` | Estimated trace counts within limits |
| `DeniedAddressValidator` | Sender, recipient, and EIP-7702 authorization list (authority + delegation address) deny list check |
| `PrecompileAddressValidator` | Excluded precompile address check |
| `GasLimitValidator` | Gas limit below configured maximum |
| `CalldataValidator` | Calldata size below limit (default 30,000 bytes) |
| `ProfitabilityValidator` | Minimum fee/size profitability ratio |
| `SimulationValidator` | Transaction simulation passes |

## Priority Transactions

Transactions from addresses in a predefined priority list are processed before normal transactions. These typically correspond to Linea system transactions (message anchoring, forced transactions).

## Gas Estimation

The sequencer exposes `linea_estimateGas` via the Besu plugin, which accounts for Linea-specific costs (L1 data cost, compression ratio) beyond standard `eth_estimateGas`. Gas pricing data is propagated via block header `extraData` fields.

## Transaction Bundles

Linea supports atomic transaction bundles via two custom JSON-RPC methods:

- `linea_sendBundle` — Submit a bundle of raw signed transactions targeting a specific block number. All transactions must be individually valid; if any fails, the entire bundle is reverted.
- `linea_cancelBundle` — Cancel a previously submitted bundle by its bundle hash.

The `BundleConstraintTransactionSelector` and `MaxBundleGasPerBlockTransactionSelector` enforce bundle atomicity and per-block gas limits during block building.

## Test Coverage

| Test File | Runner | Validates |
|-----------|--------|-----------|
| `besu-plugins/linea-sequencer/sequencer/` unit tests | JUnit 5 | Selectors, validators, trace limits |
| `besu-plugins/linea-sequencer/acceptance-tests/` | Jest | End-to-end sequencer behavior |
| `e2e/src/l2.spec.ts` | Jest | Calldata limits, legacy/EIP-1559 tx types |
| `e2e/src/opcodes.spec.ts` | Jest | `linea_estimateGas`, opcode execution |
| `e2e/src/liveness.spec.ts` | Jest | Sequencer restart and liveness |
| `e2e/src/send-bundle.spec.ts` | Jest | Bundle inclusion, partial-invalid rejection, cancellation |

## Related Documentation

- [Architecture: Sequencer](../architecture-description.md#sequencer)
- [Architecture: Gas Price Setting](../architecture-description.md#gas-price-setting)
- [Tech: Besu Plugins Component](../tech/components/besu-plugins.md) — Shared plugin infrastructure, lifecycle, configuration
- [Official docs: Sequencer](https://docs.linea.build/protocol/architecture/sequencer)
