package linea.ethapi

import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Failed requests with JSON-RPC error responses will be rejected with JsonRpcErrorResponseException
 */
interface EthApiClient : EthLogsClient {
  fun getChainId(): SafeFuture<ULong>

  fun findBlockByNumber(
    blockParameter: BlockParameter,
  ): SafeFuture<Block?>

  fun getBlockByNumber(
    blockParameter: BlockParameter,
  ): SafeFuture<Block> {
    return findBlockByNumber(blockParameter).thenApply { block ->
      block ?: throw IllegalArgumentException("block=$blockParameter not found!")
    }
  }

  fun findBlockByNumberWithoutTransactionsData(
    blockParameter: BlockParameter,
  ): SafeFuture<BlockWithTxHashes?>

  fun getBlockByNumberWithoutTransactionsData(
    blockParameter: BlockParameter,
  ): SafeFuture<BlockWithTxHashes> {
    return findBlockByNumberWithoutTransactionsData(blockParameter).thenApply { block ->
      block ?: throw IllegalArgumentException("block=$blockParameter not found!")
    }
  }
}
