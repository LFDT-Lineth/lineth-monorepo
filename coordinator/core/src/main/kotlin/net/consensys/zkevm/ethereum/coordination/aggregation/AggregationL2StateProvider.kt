package net.consensys.zkevm.ethereum.coordination.aggregation

import kotlinx.datetime.Instant
import linea.contract.l2.L2MessageServiceSmartContractClientReadOnly
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.ethapi.EthApiClient
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface AggregationL2StateProvider {
  fun getAggregationL2State(blockNumber: Long): SafeFuture<AggregationL2State>
}

class AggregationL2StateProviderImpl(
  private val ethApiClient: EthApiClient,
  private val messageService: L2MessageServiceSmartContractClientReadOnly,
) : AggregationL2StateProvider {

  private data class AnchoredMessage(
    val messageNumber: ULong,
    val rollingHash: ByteArray,
  )

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

  override fun getAggregationL2State(blockNumber: Long): SafeFuture<AggregationL2State> {
    val blockParameter = blockNumber.toBlockParameter()
    return getLastAnchoredMessage(blockNumber.toULong())
      .thenCombine(
        ethApiClient.getBlockByNumberWithoutTransactionsData(blockParameter),
      ) { (messageNumber, rollingHash), block ->
        AggregationL2State(
          parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(block.timestamp.toLong()),
          parentAggregationLastL1RollingHashMessageNumber = messageNumber,
          parentAggregationLastL1RollingHash = rollingHash,
        )
      }
  }
}
