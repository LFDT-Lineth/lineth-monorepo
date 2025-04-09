package net.consensys.zkevm.ethereum.coordination.proofcreation

import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.web3j.domain.toWeb3j
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.coordinator.clients.BatchExecutionProofRequestV1
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.coordinator.clients.L2MessageServiceLogsClient
import net.consensys.zkevm.domain.Batch
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.ethereum.coordination.conflation.BlocksTracesConflated
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture

class ZkProofCreationCoordinatorImpl(
  private val executionProverClient: ExecutionProverClientV2,
  private val l2MessageServiceLogsClient: L2MessageServiceLogsClient,
  private val l2Web3jClient: Web3j
) : ZkProofCreationCoordinator {
  private val log: Logger = LogManager.getLogger(this::class.java)

  private fun getBlockStateRootHash(blockNumber: ULong): SafeFuture<String> {
    return l2Web3jClient
      .ethGetBlockByNumber(
        blockNumber.toBlockParameter().toWeb3j(),
        false
      )
      .sendAsync()
      .thenApply { block -> block.block.stateRoot }
      .toSafeFuture()
  }

  override fun createZkProof(
    blocksConflation: BlocksConflation,
    traces: BlocksTracesConflated
  ): SafeFuture<Batch> {
    val startBlockNumber = blocksConflation.blocks.first().number
    val endBlockNumber = blocksConflation.blocks.last().number
    val blocksConflationInterval = blocksConflation.intervalString()
    val bridgeLogsListFutures = blocksConflation.blocks.map { block ->
      l2MessageServiceLogsClient.getBridgeLogs(blockNumber = block.number.toLong())
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
                keccakParentStateRootHash = previousKeccakStateRootHash.encodeToByteArray()
              )
            ).thenApply {
              Batch(
                startBlockNumber = startBlockNumber,
                endBlockNumber = endBlockNumber
              )
            }.whenException {
              log.error("Prover returned for batch={} errorMessage={}", blocksConflationInterval, it.message, it)
            }
          }
      }
  }
}
