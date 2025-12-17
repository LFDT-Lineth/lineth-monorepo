package linea.web3j.ethapi

import io.vertx.core.Vertx
import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import linea.domain.EthLog
import linea.domain.FeeHistory
import linea.domain.RetryConfig
import linea.domain.Transaction
import linea.domain.TransactionForEthCall
import linea.domain.TransactionReceipt
import linea.ethapi.EthApiClient
import linea.ethapi.StateOverride
import net.consensys.linea.async.AsyncRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
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

  override fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>,
  ): SafeFuture<List<EthLog>> {
    return retry { ethApiClient.getLogs(fromBlock, toBlock, address, topics) }
  }

  override fun ethChainId(): SafeFuture<ULong> {
    return retry { ethApiClient.ethChainId() }
  }

  override fun ethProtocolVersion(): SafeFuture<Int> {
    return retry { ethApiClient.ethProtocolVersion() }
  }

  override fun ethCoinbase(): SafeFuture<ByteArray> {
    return retry { ethApiClient.ethCoinbase() }
  }

  override fun ethMining(): SafeFuture<Boolean> {
    return retry { ethApiClient.ethMining() }
  }

  override fun ethGasPrice(): SafeFuture<BigInteger> {
    return retry { ethApiClient.ethGasPrice() }
  }

  override fun ethMaxPriorityFeePerGas(): SafeFuture<BigInteger> {
    return retry { ethApiClient.ethMaxPriorityFeePerGas() }
  }

  override fun ethFeeHistory(
    blockCount: Int,
    newestBlock: BlockParameter,
    rewardPercentiles: List<Double>,
  ): SafeFuture<FeeHistory> {
    return retry(stopRetriesPredicateForTag(newestBlock)) {
      ethApiClient.ethFeeHistory(blockCount, newestBlock, rewardPercentiles)
    }
  }

  override fun ethBlockNumber(): SafeFuture<ULong> {
    return retry { ethApiClient.ethBlockNumber() }
  }

  override fun ethGetBalance(address: ByteArray, blockParameter: BlockParameter): SafeFuture<BigInteger> {
    return retry(stopRetriesPredicateForTag(blockParameter)) {
      ethApiClient.ethGetBalance(address, blockParameter)
    }
  }

  override fun ethGetTransactionCount(address: ByteArray, blockParameter: BlockParameter): SafeFuture<ULong> {
    return retry(stopRetriesPredicateForTag(blockParameter)) {
      ethApiClient.ethGetTransactionCount(address, blockParameter)
    }
  }

  override fun ethFindBlockByNumberFullTxs(blockParameter: BlockParameter): SafeFuture<Block?> {
    return retry(stopRetriesPredicateForTag(blockParameter)) {
      ethApiClient.ethFindBlockByNumberFullTxs(blockParameter)
    }
  }

  override fun ethFindBlockByNumberTxHashes(blockParameter: BlockParameter): SafeFuture<BlockWithTxHashes?> {
    return retry(stopRetriesPredicateForTag(blockParameter)) {
      ethApiClient.ethFindBlockByNumberTxHashes(blockParameter)
    }
  }

  override fun ethGetTransactionByHash(transactionHash: ByteArray): SafeFuture<Transaction?> {
    return retry { ethApiClient.ethGetTransactionByHash(transactionHash) }
  }

  override fun ethGetTransactionReceipt(transactionHash: ByteArray): SafeFuture<TransactionReceipt?> {
    return retry { ethApiClient.ethGetTransactionReceipt(transactionHash) }
  }

  override fun ethSendRawTransaction(signedTransactionData: ByteArray): SafeFuture<ByteArray> {
    return retry { ethApiClient.ethSendRawTransaction(signedTransactionData) }
  }

  override fun ethCall(
    transaction: TransactionForEthCall,
    blockParameter: BlockParameter,
    stateOverride: StateOverride?,
  ): SafeFuture<ByteArray> {
    val predicate = stopRetriesPredicateForTag(blockParameter).or(stopOnExecutionRevertedPredicate)
    return retry(predicate) {
      ethApiClient.ethCall(transaction, blockParameter, stateOverride)
    }
  }

  val stopOnExecutionRevertedPredicate: Predicate<Throwable> = Predicate { th ->

    if (th.cause is linea.error.JsonRpcErrorResponseException) {
      val rpcError = th.cause as linea.error.JsonRpcErrorResponseException
      // -32000 = "execution reverted", no point in retrying
      rpcError.rpcErrorCode == -32000 ||
        rpcError.rpcErrorMessage.contains("reverted", ignoreCase = true) ||
        rpcError.rpcErrorData.toString().contains("reverted", ignoreCase = true)
    } else {
      false
    }
  }

  override fun ethEstimateGas(transaction: TransactionForEthCall): SafeFuture<ULong> {
    return retry { ethApiClient.ethEstimateGas(transaction) }
  }
}
