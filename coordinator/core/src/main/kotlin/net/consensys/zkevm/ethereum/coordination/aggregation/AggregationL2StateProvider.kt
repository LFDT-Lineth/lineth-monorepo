package net.consensys.zkevm.ethereum.coordination.aggregation

import linea.contract.l2.L2MessageServiceSmartContractClientReadOnly
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.ethapi.EthApiClient
import linea.persistence.ForcedTransactionsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

interface AggregationL2StateProvider {
  fun getAggregationL2State(blockNumber: Long): SafeFuture<AggregationL2State>
}

class AggregationL2StateProviderImpl(
  private val ethApiClient: EthApiClient,
  private val messageService: L2MessageServiceSmartContractClientReadOnly,
  private val forcedTransactionsDao: ForcedTransactionsDao,
) : AggregationL2StateProvider {
  private data class AnchoredMessage(
    val messageNumber: ULong,
    val rollingHash: ByteArray,
  )

  private data class FtxRollingInfo(
    val ftxNumber: ULong,
    val ftxRollingHash: ByteArray,
  ) {
    companion object {
      val GENESIS = FtxRollingInfo(0uL, ByteArray(32))
    }
  }

  private fun getLastAnchoredMessage(blockNumber: ULong): SafeFuture<AnchoredMessage> {
    return messageService
      .getDeploymentBlock()
      .thenCompose { deploymentBlockNumber ->
        if (blockNumber < deploymentBlockNumber) {
          // this happens always at 1st conflation, where the block number is 0
          // will happen until message service is deployed
          SafeFuture.completedFuture(AnchoredMessage(0UL, ByteArray(32)))
        } else {
          messageService
            .getLastAnchoredL1MessageNumber(block = blockNumber.toBlockParameter())
            .thenCompose { lastAnchoredMessageNumber ->
              messageService.getRollingHashByL1MessageNumber(
                block = blockNumber.toBlockParameter(),
                l1MessageNumber = lastAnchoredMessageNumber,
              )
                .thenApply { rollingHash -> AnchoredMessage(lastAnchoredMessageNumber, rollingHash) }
            }
        }
      }
  }

  private fun getAggregationFtxRollingInfo(aggEndBlockNumber: ULong): SafeFuture<FtxRollingInfo> {
    if (aggEndBlockNumber == 0uL) {
      // return genesis ftx number and hash
      return SafeFuture.completedFuture(FtxRollingInfo.GENESIS)
    }

    return forcedTransactionsDao
      .findHighestForcedTransaction(
        upToSimulatedExecutionBlockNumberInclusive = aggEndBlockNumber.minus(1uL),
      ).thenApply { highestFtx ->
        highestFtx
          ?.let { FtxRollingInfo(it.ftxNumber, it.ftxRollingHash) }
          ?: FtxRollingInfo.GENESIS
      }
  }

  override fun getAggregationL2State(blockNumber: Long): SafeFuture<AggregationL2State> {
    val blockParameter = blockNumber.toBlockParameter()
    val anchoredMessageFuture = getLastAnchoredMessage(blockNumber.toULong())
    val aggregationFtxNumbersFuture = getAggregationFtxRollingInfo(blockNumber.toULong())
    val blockFuture = ethApiClient.ethGetBlockByNumberTxHashes(blockParameter)

    return SafeFuture
      .allOf(anchoredMessageFuture, aggregationFtxNumbersFuture, blockFuture)
      .thenApply {
        val (messageNumber, rollingHash) = anchoredMessageFuture.get()
        val block = blockFuture.get()
        val (ftxNumber, ftxRollingHash) = aggregationFtxNumbersFuture.get()
        AggregationL2State(
          parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(block.timestamp.toLong()),
          parentAggregationLastL1RollingHashMessageNumber = messageNumber,
          parentAggregationLastL1RollingHash = rollingHash,
          parentAggregationLastFtxNumber = ftxNumber,
          parentAggregationLastFtxRollingHash = ftxRollingHash,
        )
      }
  }
}
