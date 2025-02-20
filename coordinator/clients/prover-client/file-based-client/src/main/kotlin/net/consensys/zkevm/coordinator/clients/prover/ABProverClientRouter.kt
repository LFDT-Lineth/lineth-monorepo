package net.consensys.zkevm.coordinator.clients.prover

import linea.domain.BlockInterval
import net.consensys.zkevm.coordinator.clients.ProverClient
import tech.pegasys.teku.infrastructure.async.SafeFuture

class StartBlockNumberBasedSwitchPredicate<ProofRequest>(
  private val switchStartBlockNumberInclusive: ULong
) where ProofRequest : BlockInterval {
  fun invoke(proofRequest: ProofRequest): Boolean = proofRequest.startBlockNumber >= switchStartBlockNumberInclusive
}

class ABProverClientRouter<ProofRequest, ProofResponse>(
  private val proverA: ProverClient<ProofRequest, ProofResponse>,
  private val proverB: ProverClient<ProofRequest, ProofResponse>,
  private val switchToProverBPredicate: (ProofRequest) -> Boolean
) : ProverClient<ProofRequest, ProofResponse> {

  override fun requestProof(proofRequest: ProofRequest): SafeFuture<ProofResponse> {
    return if (switchToProverBPredicate(proofRequest)) {
      proverB.requestProof(proofRequest)
    } else {
      proverA.requestProof(proofRequest)
    }
  }
}
