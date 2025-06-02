package net.consensys.zkevm.ethereum.coordination.proofcreation

import linea.contract.events.L1L2MessageHashesAddedToInboxEvent
import linea.contract.events.L2RollingHashUpdatedEvent
import linea.contract.events.MessageSentEvent
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.EthLog
import linea.ethapi.EthApiClient
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofRequestV1
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.domain.Batch
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.ethereum.coordination.conflation.BlocksTracesConflated
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class ZkProofCreationCoordinatorImpl(
  private val executionProverClient: ExecutionProverClientV2,
  private val messageServiceAddress: String,
  private val l2EthApiClient: EthApiClient,
) : ZkProofCreationCoordinator {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private val messageEventsTopics: List<String> = listOf(
    MessageSentEvent.topic,
    L1L2MessageHashesAddedToInboxEvent.topic,
    L2RollingHashUpdatedEvent.topic,
  )

  private fun getBlockStateRootHash(blockNumber: ULong): SafeFuture<ByteArray> {
    return l2EthApiClient
      .getBlockByNumberWithoutTransactionsData(blockNumber.toBlockParameter())
      .thenApply { block -> block.stateRoot }
  }

  private fun getBridgeLogs(blockNumber: ULong): SafeFuture<List<EthLog>> {
    return messageEventsTopics
      .map { messageEventTopic ->
        l2EthApiClient.getLogs(
          fromBlock = blockNumber.toBlockParameter(),
          toBlock = blockNumber.toBlockParameter(),
          address = messageServiceAddress,
          topics = listOf(messageEventTopic),
        )
      }.let {
        SafeFuture.collectAll(it.stream()).thenApply { it.flatten() }
      }
  }

  override fun createZkProof(
    blocksConflation: BlocksConflation,
    traces: BlocksTracesConflated,
  ): SafeFuture<Batch> {
    val startBlockNumber = blocksConflation.blocks.first().number
    val endBlockNumber = blocksConflation.blocks.last().number
    val blocksConflationInterval = blocksConflation.intervalString()
    val bridgeLogsListFutures = blocksConflation.blocks.map { block ->
      getBridgeLogs(block.number)
    }

    return getBlockStateRootHash(blocksConflation.startBlockNumber - 1UL)
      .thenCompose { previousKeccakStateRootHash ->
        SafeFuture.collectAll(bridgeLogsListFutures.stream())
          .thenCompose { bridgeLogsList ->
            executionProverClient.requestProof(
              BatchExecutionProofRequestV1(
                blocks = blocksConflation.blocks,
                bridgeLogs = bridgeLogsList.flatten(),
                tracesResponse = traces.tracesResponse,
                type2StateData = traces.zkStateTraces,
                keccakParentStateRootHash = previousKeccakStateRootHash,
              ),
            ).thenApply {
              Batch(
                startBlockNumber = startBlockNumber,
                endBlockNumber = endBlockNumber,
              )
            }.whenException {
              log.error("Prover returned for batch={} errorMessage={}", blocksConflationInterval, it.message, it)
            }
          }
      }
  }
}
