# Execution witness JSON-RPC client

Kotlin client for Besu's `debug_executionWitness` RPC (provided by [besu-zkevm-plugin](https://github.com/Consensys/besu-zkevm-plugin)).

## Interface

[`ExecutionWitnessClient`](../../../jvm-libs/linea/clients/interfaces/src/main/kotlin/linea/executionwitness/ExecutionWitnessClient.kt) in `jvm-libs:linea:clients:interfaces`.

Implementation: [`ExecutionWitnessJsonRpcClient`](src/main/kotlin/linea/coordinator/clients/executionwitness/ExecutionWitnessJsonRpcClient.kt).

## RPC

**Request**

```json
{
  "jsonrpc": "2.0",
  "method": "debug_executionWitness",
  "params": ["<block>"],
  "id": 1
}
```

**Response** (`result` object, or `null` if witness unavailable):

```json
{
  "state": ["..."],
  "keys": ["..."],
  "codes": ["..."],
  "headers": ["..."]
}
```

Hex strings may be with or without a `0x` prefix.

## Block parameter mapping

| `BlockParameter` | RPC `params[0]` |
|------------------|-----------------|
| `Tag.LATEST` (etc.) | tag string |
| `BlockNumber(n)` | decimal `n.toString()` |
| `BlockHash(h)` | `0x` + hex (32 bytes) |

## Node prerequisites

- `besu-zkevm-plugin` loaded
- Trie-log plugin enabled
- `--rpc-http-api=DEBUG`

## Tests

```bash
./gradlew :coordinator:clients:execution-witness-client:test
```
