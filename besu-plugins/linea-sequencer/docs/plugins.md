# Linea plugins

## Shared components

### Profitability calculator
The profitability calculator is a shared component, that is used to check if a tx is profitable.
It's applied, with different configuration to:
1. `linea_estimateGas` endpoint
2. Tx validation for the txpool (if tx profitability check is enabled)
3. Tx selection during block creation

#### CLI options

| Command Line Argument                                 | Default Value |
|-------------------------------------------------------|---------------|
| `--plugin-linea-fixed-gas-cost-wei`                   | 0             |
| `--plugin-linea-variable-gas-cost-wei`                | 1_000_000_000 |
| `--plugin-linea-extra-data-pricing-enabled`           | false         |
| `--plugin-linea-min-margin`                           | 1.0           |
| `--plugin-linea-estimate-gas-min-margin`              | 1.0           |
| `--plugin-linea-tx-pool-min-margin`                   | 0.5           |
| `--plugin-linea-extra-data-set-min-gas-price-enabled` | true          |


### Module line count validator
The Module line count validator is a shared component, that is used to check if a tx exceeds any of the configured line count limits.
It is used in:
1. `linea_estimateGas` endpoint
2. Tx validation for the txpool (if tx simulation is enabled)
3. Tx selection during block creation

#### CLI options

| Command Line Argument                                 | Default Value        |
|-------------------------------------------------------|----------------------|
| `--plugin-linea-module-limit-file-path`               | moduleLimitFile.toml |
| `--plugin-linea-over-line-count-limit-cache-size`     | 10_000               |


### L1<>L2 bridge

These values are just passed to the ZkTracer

#### CLI Options

| Command Line Argument                  | Default Value |
|----------------------------------------|---------------|
| `--plugin-linea-l1l2-bridge-contract`  |               |
| `--plugin-linea-l1l2-bridge-topic`     |               |


## Sequencer

### Transaction selection - LineaTransactionSelectorPlugin
This plugin extends the standard transaction selection protocols employed by Besu for block creation. 
It leverages the `TransactionSelectionService` to manage and customize the process of transaction selection. 
This includes setting limits such as `TraceLineLimit`, `maxBlockGas`, and `maxCallData`, and check the profitability
of a transaction.
The selectors are in the package `net.consensys.linea.sequencer.txselection.selectors`.

#### CLI options

| Command Line Argument                                  | Default Value        |
|--------------------------------------------------------|----------------------|
| `--plugin-linea-max-block-calldata-size`               | 70000                |
| `--plugin-linea-module-limit-file-path`                | moduleLimitFile.toml |
| `--plugin-linea-over-line-count-limit-cache-size`      | 10_000               |
| `--plugin-linea-max-block-gas`                         | 30_000_000L          |
| `--plugin-linea-unprofitable-cache-size`               | 100_000              |
| `--plugin-linea-unprofitable-retry-limit`              | 10                   |


### Transaction validation - LineaTransactionPoolValidatorPlugin

This plugin extends the default transaction validation rules for adding transactions to the
transaction pool. It leverages the `PluginTransactionValidatorService` to manage and customize the
process of transaction validation.
This includes setting limits such as `TraceLineLimit`, `maxTxGasLimit`, and `maxTxCallData`, and checking the profitability
of a transaction.
The validators are in the package `net.consensys.linea.sequencer.txpoolvalidation.validators`.

#### CLI options

| Command Line Argument                                    | Default Value     |
|----------------------------------------------------------|-------------------|
| `--plugin-linea-deny-list-path`                          | lineaDenyList.txt |
| `--plugin-linea-max-tx-gas-limit`                        | 30_000_000        |
| `--plugin-linea-max-tx-calldata-size`                    | 60_000            |
| `--plugin-linea-tx-pool-simulation-check-api-enabled`    | false             |
| `--plugin-linea-tx-pool-simulation-check-p2p-enabled`    | false             |
| `--plugin-linea-tx-pool-profitability-check-api-enabled` | true              |
| `--plugin-linea-tx-pool-profitability-check-p2p-enabled` | false             |

### Transaction validation - LineaTransactionValidatorPlugin

This plugin uses Besu's `TransactionValidatorService` to filter transactions at multiple critical lifecycle stages:
1. Block import - when a Besu validator receives a block via P2P gossip
2. Transaction pool - when a Besu node adds to its local transaction pool
3. Block production - when a Besu node builds a block

The plugin implements transaction filtering for blob transactions (EIP-4844) and delegate code transactions (EIP-7702), which can be enabled or disabled via configuration. In the future, this plugin may consolidate logic from other transaction-filtering plugins to unify transaction filtering logic.

#### CLI options

| Command Line Argument                     | Default Value |
|-------------------------------------------|---------------|
| `--plugin-linea-blob-tx-enabled`          | false         |
| `--plugin-linea-delegate-code-tx-enabled` | false         |

### Reporting rejected transactions 
The transaction selection and validation plugins can report rejected transactions as JSON-RPC calls to an external 
service. This feature can be enabled by setting the following CLI options:

| Command Line Argument                 | Default Value | Expected Values                                              |
|---------------------------------------|---------------|--------------------------------------------------------------|
| `--plugin-linea-rejected-tx-endpoint` | `null`        | A valid URL e.g. `http://localhost:9363` to enable reporting |
| `--plugin-linea-node-type`            | `null`        | One of `SEQUENCER`, `RPC`, `P2P`                             |

## RPC methods

### Linea Estimate Gas
#### `linea_estimateGas`

This endpoint simulates a transaction, including line count limit validation, and returns the estimated gas used 
(as the standard `eth_estimateGas` with `strict=true`) plus the estimated gas price to be used when submitting the tx. 

#### Parameters
same as `eth_estimateGas`

#### Result
```json
{
  "jsonrpc": "2.0",
  "id": 53,
  "result": {
    "gasLimit": "0x5208",
    "baseFeePerGas": "0x7",
    "priorityFeePerGas": "0x123456"
  }
}
```

### Linea Set Extra Data
#### `linea_setExtraData`

This endpoint is used to configure the extra data based pricing, and it only makes sense to call it on the sequencer.
Internally it sets runtime pricing configuration and then calls, via the in-process RPC service, `miner_setExtraData`
and `miner_setMinGasPrice` to update internal Besu configuration, and add the extra data pricing to the future built blocks.

#### Parameters
same as `miner_setExtraData` with the added constraint that the number of bytes must be 32

#### Result
```json
{
  "jsonrpc": "2.0",
  "id": 53,
  "result": "true"
}
```

