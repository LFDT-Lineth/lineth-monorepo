package linea.web3j.ethapi

import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import linea.domain.EthLog
import linea.domain.FeeHistory
import linea.domain.Transaction
import linea.domain.TransactionForEthCall
import linea.domain.TransactionReceipt
import linea.ethapi.EthApiClient
import linea.ethapi.StateOverride
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.kotlin.toULong
import linea.web3j.EthFeeHistoryBlobExtended
import linea.web3j.domain.toDomain
import linea.web3j.domain.toWeb3j
import linea.web3j.getWeb3jService
import linea.web3j.mappers.mapToDomainWithTxHashes
import linea.web3j.mappers.toDomain
import linea.web3j.mappers.toWeb3j
import linea.web3j.requestAsync
import org.web3j.protocol.Web3j
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.Log
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import kotlin.jvm.optionals.getOrNull

/**
 * Web3j Adapter of EthApiClient
 * Request retries is responsibility of another class
 */
class Web3jEthApiClient(
  val web3jClient: Web3j,
  val web3jService: Web3jService = web3jClient.getWeb3jService(),
) : EthApiClient {
  override fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>,
  ): SafeFuture<List<EthLog>> {
    val ethFilter = EthFilter(
      /*fromBlock*/
      fromBlock.toWeb3j(),
      /*toBlock*/
      toBlock.toWeb3j(),
      /*address*/
      address,
    ).apply {
      topics.forEach { addSingleTopic(it) }
    }

    return web3jClient
      .ethGetLogs(ethFilter)
      .requestAsync { logsResponse ->
        if (logsResponse.logs != null) {
          @Suppress("UNCHECKED_CAST")
          (logsResponse.logs as List<org.web3j.protocol.core.methods.response.EthLog.LogResult<Log>>)
            .map { logResult -> logResult.get().toDomain() }
        } else {
          emptyList()
        }
      }
  }

  override fun ethChainId(): SafeFuture<ULong> = web3jClient.ethChainId().requestAsync { it.chainId.toULong() }

  override fun ethProtocolVersion(): SafeFuture<Int> =
    web3jClient.ethProtocolVersion().requestAsync { it.protocolVersion.toInt() }

  override fun ethCoinbase(): SafeFuture<ByteArray> = web3jClient.ethCoinbase().requestAsync { it.address.decodeHex() }

  override fun ethMining(): SafeFuture<Boolean> = web3jClient.ethMining().requestAsync { it.isMining }

  override fun ethGasPrice(): SafeFuture<BigInteger> = web3jClient.ethGasPrice().requestAsync { it.gasPrice }

  override fun ethMaxPriorityFeePerGas(): SafeFuture<BigInteger> =
    web3jClient.ethMaxPriorityFeePerGas().requestAsync { it.maxPriorityFeePerGas }

  override fun ethFeeHistory(
    blockCount: Int,
    newestBlock: BlockParameter,
    rewardPercentiles: List<Double>,
  ): SafeFuture<FeeHistory> {
    return Request(
      "eth_feeHistory",
      listOf(
        Numeric.encodeQuantity(BigInteger.valueOf(blockCount.toLong())),
        newestBlock.toWeb3j().value,
        rewardPercentiles,
      ),
      this.web3jService,
      EthFeeHistoryBlobExtended::class.java,
    ).requestAsync {
      it.feeHistory.toLineaDomain()
    }
  }

  override fun ethBlockNumber(): SafeFuture<ULong> = web3jClient.ethBlockNumber().requestAsync {
    it.blockNumber?.toULong() ?: throw IllegalStateException("Block number not found in response")
  }

  override fun ethGetBalance(address: ByteArray, blockParameter: BlockParameter): SafeFuture<BigInteger> = web3jClient
    .ethGetBalance(address.encodeHex(), blockParameter.toWeb3j())
    .requestAsync { it.balance }

  override fun ethGetTransactionCount(address: ByteArray, blockParameter: BlockParameter): SafeFuture<ULong> =
    web3jClient
      .ethGetTransactionCount(address.encodeHex(), blockParameter.toWeb3j())
      .requestAsync { it.transactionCount.toULong() }

  override fun ethFindBlockByNumberFullTxs(blockParameter: BlockParameter): SafeFuture<Block?> {
    return web3jClient
      .ethGetBlockByNumber(blockParameter.toWeb3j(), true)
      .requestAsync { resp -> resp.block?.toDomain() }
  }

  override fun ethFindBlockByNumberTxHashes(blockParameter: BlockParameter): SafeFuture<BlockWithTxHashes?> {
    return web3jClient
      .ethGetBlockByNumber(blockParameter.toWeb3j(), false)
      .requestAsync { resp -> resp.block?.let(::mapToDomainWithTxHashes) }
  }

  override fun ethGetTransactionByHash(transactionHash: ByteArray): SafeFuture<Transaction?> {
    return web3jClient
      .ethGetTransactionByHash(transactionHash.encodeHex())
      .requestAsync { resp -> resp.transaction?.getOrNull()?.toDomain() }
  }

  override fun ethGetTransactionReceipt(transactionHash: ByteArray): SafeFuture<TransactionReceipt?> {
    return web3jClient
      .ethGetTransactionReceipt(transactionHash.encodeHex())
      .requestAsync { resp -> resp.transactionReceipt?.getOrNull()?.toDomain() }
  }

  override fun ethSendRawTransaction(signedTransactionData: ByteArray): SafeFuture<ByteArray> {
    return web3jClient
      .ethSendRawTransaction(signedTransactionData.encodeHex())
      .requestAsync { resp -> resp.transactionHash.decodeHex() }
  }

  override fun ethCall(
    transaction: TransactionForEthCall,
    blockParameter: BlockParameter,
    stateOverride: StateOverride?,
  ): SafeFuture<ByteArray> {
    require(stateOverride == null) { "web3j eth_call does not support stateOverrides" }
    return web3jClient
      .ethCall(transaction.toWeb3j(), blockParameter.toWeb3j())
      .requestAsync { resp -> resp.value.decodeHex() }
  }

  override fun ethEstimateGas(transaction: TransactionForEthCall): SafeFuture<ULong> {
    return web3jClient
      .ethEstimateGas(transaction.toWeb3j())
      .requestAsync { resp -> resp.amountUsed.toULong() }
  }
}
