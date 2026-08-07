package lineth.coordination.proofcreation

import lineth.clients.BatchExecutionProofRequestV1
import lineth.clients.ExecutionProverClientV2
import lineth.contract.events.L1L2MessageHashesAddedToInboxEvent
import lineth.contract.events.L2RollingHashUpdatedEvent
import lineth.contract.events.MessageSentEvent
import lineth.coordination.conflation.BlocksTracesConflated
import lineth.domain.BlocksConflation
import lineth.domain.EthLog
import lineth.domain.ExecutionProofIndex
import lineth.domain.toBlockParameter
import lineth.ethapi.EthApiClient
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class ZkProofCreationCoordinatorImpl(
  private val executionProverClient: ExecutionProverClientV2,
  private val messageServiceAddress: String,
  private val l2EthApiClient: EthApiClient,
  private val log: Logger = LogManager.getLogger(ZkProofCreationCoordinatorImpl::class.java),
) : ZkProofCreationCoordinator {
  private val messageEventsTopics: List<String> =
    listOf(
      MessageSentEvent.topic,
      L1L2MessageHashesAddedToInboxEvent.topic,
      L2RollingHashUpdatedEvent.topic,
    )

  private fun getBlockStateRootHash(blockNumber: ULong): SafeFuture<ByteArray> {
    return l2EthApiClient
      .ethGetBlockByNumberTxHashes(blockNumber.toBlockParameter())
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

  override fun createZkProofRequest(
    blocksConflation: BlocksConflation,
    traces: BlocksTracesConflated,
  ): SafeFuture<ExecutionProofIndex> {
    val blocksConflationInterval = blocksConflation.intervalString()
    val bridgeLogsListFutures =
      blocksConflation.blocks.map { block ->
        getBridgeLogs(block.number)
      }

    return getBlockStateRootHash(blocksConflation.startBlockNumber - 1UL)
      .thenCompose { previousKeccakStateRootHash ->
        SafeFuture.collectAll(bridgeLogsListFutures.stream())
          .thenCompose { bridgeLogsList ->
            executionProverClient.createProofRequest(
              BatchExecutionProofRequestV1(
                blocks = blocksConflation.blocks,
                bridgeLogs = bridgeLogsList.flatten(),
                tracesResponse = traces.tracesResponse,
                type2StateData = traces.zkStateTraces,
                keccakParentStateRootHash = previousKeccakStateRootHash,
              ),
            ).whenException {
              log.error(
                "Prover request creation failed for batch={} errorMessage={}",
                blocksConflationInterval,
                it.message,
                it,
              )
            }
          }
      }
  }

  override fun isZkProofRequestProven(proofIndex: ExecutionProofIndex): SafeFuture<Boolean> =
    executionProverClient.isProofAlreadyDone(proofIndex)
}
