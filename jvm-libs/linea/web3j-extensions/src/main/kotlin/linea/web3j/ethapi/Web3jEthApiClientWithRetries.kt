package linea.web3j.ethapi

import io.vertx.core.Vertx
import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import linea.domain.EthLog
import linea.domain.RetryConfig
import linea.ethapi.EthApiClient
import net.consensys.linea.async.AsyncRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture

class Web3jEthApiClientWithRetries(
  val vertx: Vertx,
  val ethApiClient: EthApiClient,
  val requestRetryConfig: RetryConfig
) : EthApiClient {

  private fun <T> retry(
    fn: () -> SafeFuture<T>
  ): SafeFuture<T> {
    return AsyncRetryer.retry(
      vertx = vertx,
      backoffDelay = requestRetryConfig.backoffDelay,
      timeout = requestRetryConfig.timeout,
      maxRetries = requestRetryConfig.maxRetries?.toInt(),
      action = fn
    )
  }

  override fun findBlockByNumber(blockParameter: BlockParameter): SafeFuture<Block?> {
    return retry { ethApiClient.findBlockByNumber(blockParameter) }
  }

  override fun findBlockByNumberWithoutTransactionsData(
    blockParameter: BlockParameter
  ): SafeFuture<BlockWithTxHashes?> {
    return retry { ethApiClient.findBlockByNumberWithoutTransactionsData(blockParameter) }
  }

  override fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>
  ): SafeFuture<List<EthLog>> {
    return retry { ethApiClient.getLogs(fromBlock, toBlock, address, topics) }
  }
}
