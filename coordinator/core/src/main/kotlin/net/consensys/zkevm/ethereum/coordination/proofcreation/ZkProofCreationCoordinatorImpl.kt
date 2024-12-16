package net.consensys.zkevm.ethereum.coordination.proofcreation

import net.consensys.zkevm.coordinator.clients.BatchExecutionProofRequestV1
import net.consensys.zkevm.coordinator.clients.ExecutionProverClientV2
import net.consensys.zkevm.domain.Batch
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.ethereum.coordination.conflation.BlocksTracesConflated
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class ZkProofCreationCoordinatorImpl(
  private val executionProverClient: ExecutionProverClientV2
) : ZkProofCreationCoordinator {
  private val log: Logger = LogManager.getLogger(this::class.java)

  override fun createZkProof(
    blocksConflation: BlocksConflation,
    traces: BlocksTracesConflated
  ): SafeFuture<Batch> {
    val startBlockNumber = blocksConflation.blocks.first().number.toULong()
    val endBlockNumber = blocksConflation.blocks.last().number.toULong()
    val blocksConflationInterval = blocksConflation.intervalString()

    return executionProverClient
      .requestProof(BatchExecutionProofRequestV1(blocksConflation.blocks, traces.tracesResponse, traces.zkStateTraces))
      .thenApply {
        Batch(
          startBlockNumber = startBlockNumber,
          endBlockNumber = endBlockNumber
        )
      }
      .whenException {
        log.error("Prover returned for batch={} errorMessage={}", blocksConflationInterval, it.message, it)
      }
  }
}
