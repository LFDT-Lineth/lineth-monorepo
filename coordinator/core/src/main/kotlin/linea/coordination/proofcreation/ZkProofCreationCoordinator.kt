package linea.coordination.proofcreation

import linea.coordination.conflation.BlocksTracesConflated
import linea.domain.BlocksConflation
import linea.domain.ExecutionProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ZkProofCreationCoordinator {
  fun createZkProofRequest(
    blocksConflation: BlocksConflation,
    traces: BlocksTracesConflated,
  ): SafeFuture<ExecutionProofIndex>
  fun isZkProofRequestProven(proofIndex: ExecutionProofIndex): SafeFuture<Boolean>
}
