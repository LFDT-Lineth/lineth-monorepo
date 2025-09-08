package net.consensys.zkevm.coordinator.clients.prover

import linea.domain.BlockInterval
import net.consensys.zkevm.coordinator.clients.ProverClient
import net.consensys.zkevm.domain.ProofIndex
import tech.pegasys.teku.infrastructure.async.SafeFuture

class StartBlockNumberBasedSwitchPredicate(
  private val switchStartBlockNumberInclusive: ULong,
) {
  fun invoke(proofRequest: BlockInterval): Boolean = proofRequest.startBlockNumber >= switchStartBlockNumberInclusive
}

class ABProverClientRouter<ProofRequest, ProofResponse>(
  private val proverA: ProverClient<ProofRequest, ProofResponse>,
  private val proverB: ProverClient<ProofRequest, ProofResponse>,
  private val switchToProverBPredicate: (BlockInterval) -> Boolean,
) : ProverClient<ProofRequest, ProofResponse> where ProofRequest : BlockInterval {
  override fun findProofResponse(proofRequestId: ProofIndex): SafeFuture<ProofResponse?> {
    return if (switchToProverBPredicate(proofRequestId)) {
      proverB.findProofResponse(proofRequestId)
    } else {
      proverA.findProofResponse(proofRequestId)
    }
  }

  override fun requestProof(proofRequest: ProofRequest): SafeFuture<ProofResponse> {
    return if (switchToProverBPredicate(proofRequest)) {
      proverB.requestProof(proofRequest)
    } else {
      proverA.requestProof(proofRequest)
    }
  }
}
