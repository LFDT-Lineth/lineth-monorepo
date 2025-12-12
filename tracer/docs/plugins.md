# Linea plugins

## Shared components

### Profitability calculator
The profitability calculator is a shared component, that is used to check if a tx is profitable.
It is applied, with different configuration to:
1. `linea_estimateGas` endpoint
2. Tx validation for the txpool
3. Tx selection during block creation

#### CLI options

| Option Name              | Default Value | Command Line Argument                     |
|--------------------------|---------------|-------------------------------------------|
| L1_VERIFICATION_GAS_COST | 1_200_000     | `--plugin-linea-verification-gas-cost`    |
| L1_VERIFICATION_CAPACITY | 90_000        | `--plugin-linea-verification-capacity`    |
| L1_L2_GAS_PRICE_RATIO    | 15            | `--plugin-linea-gas-price-ratio`          |
| L2_GAS_PRICE_ADJUSTMENT  | 0 wei         | `--plugin-linea-gas-price-adjustment`     |
| MIN_MARGIN               | 1.0           | `--plugin-linea-min-margin`               |
| ESTIMATE_GAS_MIN_MARGIN  | 1.0           | `--plugin-linea-estimate-gas-min-margin`  |
| TX_POOL_MIN_MARGIN       | 0.5           | `--plugin-linea-tx-pool-min-margin`       |
| UNPROFITABLE_CACHE_SIZE  | 100_000       | `--plugin-linea-unprofitable-cache-size`  |
| UNPROFITABLE_RETRY_LIMIT | 10            | `--plugin-linea-unprofitable-retry-limit` |
| TX_POOL_ENABLE_CHECK_API     | true          | `--plugin-linea-tx-pool-profitability-check-api-enabled`  |
| TX_POOL_ENABLE_CHECK_P2P     | false         | `--plugin-linea-tx-pool-profitability-check-p2p-enabled`  |

### L1<>L2 bridge

#### CLI options

| Option Name                  | Default Value | Command Line Argument                       |
|------------------------------|---------------|---------------------------------------------|
| L1L2_BRIDGE_CONTRACT_ADDRESS |               | `--plugin-linea-l1l2-bridge-contract`       |
| L1L2_BRIDGE_LOG_TOPIC        |               | `--plugin-linea-l1l2-bridge-topic`          |

## Sequencer
### Transaction selection - LineaTransactionSelectorPlugin

This plugin extends the standard transaction selection protocols employed by Besu for block creation. 
It leverages the TransactionSelectionService to manage and customize the process of transaction selection. 
This includes setting limits such as `TraceLineLimit`, `maxBlockGas`, and `maxCallData`, and check the profitability
of a transaction.


#### CLI options

| Option Name                      | Default Value        | Command Line Argument                             |
|----------------------------------|----------------------|---------------------------------------------------|
| MAX_BLOCK_CALLDATA_SIZE          | 70000                | `--plugin-linea-max-block-calldata-size`          |
| MODULE_LIMIT_FILE_PATH           | moduleLimitFile.toml | `--plugin-linea-module-limit-file-path`           |
| OVER_LINE_COUNT_LIMIT_CACHE_SIZE | 10_000               | `--plugin-linea-over-line-count-limit-cache-size` |
| MAX_GAS_PER_BLOCK                | 30_000_000L          | `--plugin-linea-max-block-gas`                    |


### Transaction validation - LineaTransactionValidatorPlugin

This plugin extends the default transaction validation rules for adding transactions to the
transaction pool. It leverages the PluginTransactionValidatorService to manage and customize the
process of transaction validation. This includes, for example, setting a deny list of addresses
that are not allowed to add transactions to the pool.

#### CLI options

| Option Name             | Default Value     | Command Line Argument                 |
|-------------------------|-------------------|---------------------------------------|
| DENY_LIST_PATH          | lineaDenyList.txt | `--plugin-linea-deny-list-path`       |
| MAX_TX_GAS_LIMIT_OPTION | 30_000_000        | `--plugin-linea-max-tx-gas-limit`     |
| MAX_TX_CALLDATA_SIZE    | 60_000            | `--plugin-linea-max-tx-calldata-size` |

## RPC

### Linea estimate gas

#### `linea_estimateGas`

This endpoint simulates a transaction and returns the estimated gas used ( as the standard `eth_estimateGas`) plus the estimated gas price to be used when submitting the tx. 

#### Parameters

same as `eth_estimateGas`

### Counters - CountersEndpointServicePlugin
#### `rollup_getTracesCountersByBlockNumberV0` 

The CountersEndpointServicePlugin registers an RPC endpoint named `getTracesCountersByBlockNumberV0` 
under the `rollup` namespace. When this endpoint is called, returns trace counters based on the provided request parameters.

#### Parameters

  - `blockNumber`: _string_ - The block number

  - `tracerVersion`: _string_ - The tracer version. It will return an error if the 
requested version is different from the tracer runtime 

### Trace generation - TracesEndpointServicePlugin

This plugin registers an RPC endpoint named `generateConflatedTracesToFileV0` under the `rollup` namespace. 
The endpoint generates conflated file traces.

#### Parameters

- `fromBlock`: _string_ - the fromBlock number
- `toBlock`: _string_ - The toBlock number
- `tracerVersion`: _string_ - The tracer version. It will return an error if the
  requested version is different from the tracer runtime

## Continuous tracing

The continuous tracing plugin allows to trace every newly imported block and use Corset to check if the constraints are
valid. In case of an error a message will be sent to the configured Slack channel.

### Usage

The continuous tracing plugin is disabled by default. To enable it, use the `--plugin-linea-continuous-tracing-enabled`
flag. If the plugin is enabled it is mandatory to specify the location of `zkevm.bin` using
the `--plugin-linea-continuous-tracing-zkevm-bin` flag. The user with which the node is running needs to have the
appropriate permissions to access `zkevm.bin`.

In order to send a message to Slack a webhook URL needs to be specified by setting the `SLACK_SHADOW_NODE_WEBHOOK_URL`
environment variable. An environment variable was chosen instead of a command line flag to avoid leaking the webhook URL
in the process list.

The environment variable can be set via systemd using the following command:

```shell
Environment=SLACK_SHADOW_NODE_WEBHOOK_URL=https://hooks.slack.com/services/SECRET_VALUES
```

### Invalid trace handling

In the success case the trace file will simply be deleted.

In case of an error the trace file will be renamed to `trace_$BLOCK_NUMBER_$BLOCK_HASH.lt` and moved
to `$BESU_USER_HOME/invalid-traces`. The output of Corset will be saved in the same directory in a file
named `corset_output_$BLOCK_NUMBER_$BLOCK_HASH.txt`. After that an error message will be sent to Slack.
