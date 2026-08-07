package linea.coordinator.clients.prover

import lineth.clients.BatchExecutionProofRequestV1
import lineth.clients.InvalidityProofRequest
import lineth.clients.ProverClient
import lineth.domain.AggregationProofIndex
import lineth.domain.BlobCompressionProofRequest
import lineth.domain.CompressionProofIndex
import lineth.domain.ExecutionProofIndex
import lineth.domain.InvalidityProofIndex
import lineth.domain.ProofIndex
import lineth.domain.ProofsToAggregate
import lineth.domain.StartBlockTimestampProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

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

class StartBlockTimestampBasedSwitchPredicate(
  private val switchStartBlockTimestampInclusive: Instant,
) {
  fun invoke(proofRequestOrIndex: Any): Boolean {
    val startBlockTimestamp =
      (proofRequestOrIndex as? StartBlockTimestampProvider)?.startBlockTimestamp
        ?: throw IllegalArgumentException(
          "Unsupported proof request or index type: ${proofRequestOrIndex::class}",
        )
    return startBlockTimestamp >= switchStartBlockTimestampInclusive
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
