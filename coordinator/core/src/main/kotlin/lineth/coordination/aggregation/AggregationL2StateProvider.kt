package lineth.coordination.aggregation

import linea.contract.l2.L2MessageServiceSmartContractClientReadOnly
import linea.domain.toBlockParameter
import linea.ethapi.EthApiClient
import linea.kotlin.zeroHash32
import lineth.coordination.FtxRollingInfoProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

interface AggregationL2StateProvider {
  fun getAggregationL2State(blockNumber: Long): SafeFuture<AggregationL2State>
}

class AggregationL2StateProviderImpl(
  private val ethApiClient: EthApiClient,
  private val messageService: L2MessageServiceSmartContractClientReadOnly,
  private val ftxRollingInfoProvider: FtxRollingInfoProvider,
) : AggregationL2StateProvider {
  private data class AnchoredMessage(
    val messageNumber: ULong,
    val rollingHash: ByteArray,
  ) {
    companion object {
      val GENESIS = AnchoredMessage(0uL, GENESIS_ZERO_HASH.copyOf())
    }
  }

  private fun getLastAnchoredMessage(blockNumber: ULong): SafeFuture<AnchoredMessage> {
    return messageService
      .getDeploymentBlock()
      .thenCompose { deploymentBlockNumber ->
        if (blockNumber < deploymentBlockNumber) {
          // this happens always at 1st conflation, where the block number is 0
          // will happen until message service is deployed
          SafeFuture.completedFuture(AnchoredMessage.GENESIS)
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

  override fun getAggregationL2State(blockNumber: Long): SafeFuture<AggregationL2State> {
    val anchoredMessageFuture = getLastAnchoredMessage(blockNumber.toULong())
    val aggregationFtxNumbersFuture = ftxRollingInfoProvider.getParentFtxState(blockNumber.toULong())
    val blockFuture = ethApiClient.ethGetBlockByNumberTxHashes(blockNumber.toBlockParameter())

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

  companion object {
    val GENESIS_ZERO_HASH: ByteArray get() = zeroHash32()
  }
}
