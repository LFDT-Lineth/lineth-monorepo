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
import java.util.function.Predicate

class Web3jEthApiClientWithRetries(
  val vertx: Vertx,
  val ethApiClient: EthApiClient,
  val requestRetryConfig: RetryConfig,
  val stopRetriesOnErrorPredicate: Predicate<Throwable> = Predicate { _ -> false },
) : EthApiClient {

  private fun <T> retry(
    stopRetriesOnErrorPredicate: Predicate<Throwable> = this.stopRetriesOnErrorPredicate,
    fn: () -> SafeFuture<T>,
  ): SafeFuture<T> {
    return AsyncRetryer.retry(
      vertx = vertx,
      backoffDelay = requestRetryConfig.backoffDelay,
      timeout = requestRetryConfig.timeout,
      maxRetries = requestRetryConfig.maxRetries?.toInt(),
      stopRetriesOnErrorPredicate = stopRetriesOnErrorPredicate::test,
      action = fn,
    )
  }

  private fun stopRetryOnFinalizedSafeTags(th: Throwable): Boolean {
    return if (th.cause is linea.error.JsonRpcErrorResponseException) {
      val rpcError = th.cause as linea.error.JsonRpcErrorResponseException
      // 39001 = "Block Unknown", this means that the node does not support
      // SAFE/FINALIZED block tags hence retry would fail always
      rpcError.rpcErrorCode == -39001
    } else {
      false
    }
  }

  private fun stopRetriesPredicateForTag(blockParameter: BlockParameter): Predicate<Throwable> {
    return when {
      blockParameter == BlockParameter.Tag.FINALIZED ||
        blockParameter == BlockParameter.Tag.SAFE
      -> this.stopRetriesOnErrorPredicate.or(::stopRetryOnFinalizedSafeTags)

      else -> this.stopRetriesOnErrorPredicate
    }
  }

  override fun getChainId(): SafeFuture<ULong> {
    return retry { ethApiClient.getChainId() }
  }

  override fun findBlockByNumber(blockParameter: BlockParameter): SafeFuture<Block?> {
    return retry(stopRetriesPredicateForTag(blockParameter)) { ethApiClient.findBlockByNumber(blockParameter) }
  }

  override fun findBlockByNumberWithoutTransactionsData(
    blockParameter: BlockParameter,
  ): SafeFuture<BlockWithTxHashes?> {
    return retry(stopRetriesPredicateForTag(blockParameter)) {
      ethApiClient.findBlockByNumberWithoutTransactionsData(
        blockParameter,
      )
    }
  }

  override fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>,
  ): SafeFuture<List<EthLog>> {
    return retry { ethApiClient.getLogs(fromBlock, toBlock, address, topics) }
  }
}
