package lineth.coordination.proofcreation

import lineth.coordination.conflation.BlocksTracesConflated
import lineth.domain.BlocksConflation
import lineth.domain.ExecutionProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ZkProofCreationCoordinator {
  fun createZkProofRequest(
    blocksConflation: BlocksConflation,
    traces: BlocksTracesConflated,
  ): SafeFuture<ExecutionProofIndex>
  fun isZkProofRequestProven(proofIndex: ExecutionProofIndex): SafeFuture<Boolean>
}
