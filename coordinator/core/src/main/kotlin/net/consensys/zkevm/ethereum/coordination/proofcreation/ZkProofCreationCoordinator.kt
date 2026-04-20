package net.consensys.zkevm.ethereum.coordination.proofcreation

import linea.domain.BlocksConflation
import linea.domain.ExecutionProofIndex
import net.consensys.zkevm.ethereum.coordination.conflation.BlocksTracesConflated
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ZkProofCreationCoordinator {
  fun createZkProofRequest(
    blocksConflation: BlocksConflation,
    traces: BlocksTracesConflated,
  ): SafeFuture<ExecutionProofIndex>
  fun isZkProofRequestProven(proofIndex: ExecutionProofIndex): SafeFuture<Boolean>
}
