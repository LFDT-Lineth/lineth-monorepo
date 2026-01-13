package net.consensys.zkevm.ethereum.coordination.proofcreation

import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.ethereum.coordination.conflation.BlocksTracesConflated
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ZkProofCreationCoordinator {
  fun createZkProofRequest(blocksConflation: BlocksConflation, traces: BlocksTracesConflated): SafeFuture<ProofIndex>
  fun isZkProofRequestProven(proofIndex: ProofIndex): SafeFuture<Boolean>
}
