package net.consensys.zkevm.coordinator.clients.prover

import net.consensys.zkevm.coordinator.clients.BatchExecutionProofRequestV1
import net.consensys.zkevm.coordinator.clients.BlobCompressionProofRequest
import net.consensys.zkevm.coordinator.clients.InvalidityProofRequest
import net.consensys.zkevm.coordinator.clients.ProverClient
import net.consensys.zkevm.domain.AggregationProofIndex
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.domain.ExecutionProofIndex
import net.consensys.zkevm.domain.InvalidityProofIndex
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.domain.ProofsToAggregate
import tech.pegasys.teku.infrastructure.async.SafeFuture

class StartBlockNumberBasedSwitchPredicate(
  private val switchStartBlockNumberInclusive: ULong,
) {
  fun invoke(proofRequestOrIndex: Any): Boolean {
    val startBlockNumber = when (proofRequestOrIndex) {
      is BatchExecutionProofRequestV1 -> proofRequestOrIndex.startBlockNumber
      is BlobCompressionProofRequest -> proofRequestOrIndex.startBlockNumber
      is ProofsToAggregate -> proofRequestOrIndex.startBlockNumber
      is InvalidityProofRequest -> proofRequestOrIndex.simulatedExecutionBlockNumber
      is ExecutionProofIndex -> proofRequestOrIndex.startBlockNumber
      is CompressionProofIndex -> proofRequestOrIndex.startBlockNumber
      is AggregationProofIndex -> proofRequestOrIndex.startBlockNumber
      is InvalidityProofIndex -> proofRequestOrIndex.simulatedExecutionBlockNumber
      else ->
        throw IllegalArgumentException("Unsupported proof request or index type: ${proofRequestOrIndex::class}")
    }
    return startBlockNumber >= switchStartBlockNumberInclusive
  }
}

class ABProverClientRouter<ProofRequest : Any, ProofResponse, TProofIndex : ProofIndex>(
  private val proverA: ProverClient<ProofRequest, ProofResponse, TProofIndex>,
  private val proverB: ProverClient<ProofRequest, ProofResponse, TProofIndex>,
  private val switchToProverBPredicate: (Any) -> Boolean,
) : ProverClient<ProofRequest, ProofResponse, TProofIndex> {

  private fun getProver(proofRequestOrIndex: Any): ProverClient<ProofRequest, ProofResponse, TProofIndex> {
    return if (switchToProverBPredicate(proofRequestOrIndex)) {
      proverB
    } else {
      proverA
    }
  }

  override fun findProofResponse(proofIndex: TProofIndex): SafeFuture<ProofResponse?> {
    return getProver(proofIndex).findProofResponse(proofIndex)
  }

  override fun requestProof(proofRequest: ProofRequest): SafeFuture<ProofResponse> {
    return getProver(proofRequest).requestProof(proofRequest)
  }

  override fun createProofRequest(proofRequest: ProofRequest): SafeFuture<TProofIndex> {
    return getProver(proofRequest).createProofRequest(proofRequest)
  }
}
