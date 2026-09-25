package lineth.coordinator.clients.prover

import linea.clients.BatchExecutionProofRequestV1
import linea.clients.InvalidityProofRequest
import linea.clients.ProverClientV2
import linea.domain.AggregationProofIndex
import linea.domain.BlobCompressionProofRequest
import linea.domain.BlockInterval
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.domain.InvalidityProofIndex
import linea.domain.ProofIndex
import linea.domain.ProofsToAggregate
import linea.domain.StartBlockTimestampProvider
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
      is BlockInterval -> proofRequestOrIndex.startBlockNumber
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
  private val proverA: ProverClientV2<ProofRequest, ProofResponse, TProofIndex>?,
  private val proverB: ProverClientV2<ProofRequest, ProofResponse, TProofIndex>?,
  private val switchToProverBPredicate: (Any) -> Boolean,
) : ProverClientV2<ProofRequest, ProofResponse, TProofIndex> {

  companion object {
    fun <TProverConfig, ProofRequest : Any, ProofResponse, TProofIndex : ProofIndex> create(
      proverAConfig: TProverConfig?,
      proverBConfig: TProverConfig?,
      switchBlockNumberInclusive: ULong?,
      switchBlockTimestamp: Instant?,
      clientBuilder: (TProverConfig) -> ProverClientV2<ProofRequest, ProofResponse, TProofIndex>,
    ): ProverClientV2<ProofRequest, ProofResponse, TProofIndex> {
      require(proverAConfig != null || proverBConfig != null) {
        "Either proverAConfig or proverBConfig must be provided"
      }
      if (switchBlockNumberInclusive == null && switchBlockTimestamp == null) {
        requireNotNull(proverAConfig) {
          "proverAConfig must be provided if switchBlockNumberInclusive and switchBlockTimestamp are both null"
        }
      }

      return when {
        switchBlockNumberInclusive != null -> {
          ABProverClientRouter(
            proverA = proverAConfig?.let { clientBuilder(it) },
            proverB = proverBConfig?.let { clientBuilder(it) },
            switchToProverBPredicate = StartBlockNumberBasedSwitchPredicate(switchBlockNumberInclusive)::invoke,
          )
        }
        switchBlockTimestamp != null -> {
          ABProverClientRouter(
            proverA = proverAConfig?.let { clientBuilder(it) },
            proverB = proverBConfig?.let { clientBuilder(it) },
            switchToProverBPredicate = StartBlockTimestampBasedSwitchPredicate(switchBlockTimestamp)::invoke,
          )
        }
        else -> clientBuilder(proverAConfig!!)
      }
    }
  }

  private fun getProver(proofRequestOrIndex: Any): ProverClientV2<ProofRequest, ProofResponse, TProofIndex> {
    return if (switchToProverBPredicate(proofRequestOrIndex)) {
      requireNotNull(proverB) {
        "proverB should not be null, the caller should not use it after the switch"
      }
      proverB
    } else {
      requireNotNull(proverA) {
        "proverA should not be null, the caller should not use it before the switch"
      }
      proverA
    }
  }

  override fun findProofResponse(proofIndex: TProofIndex): SafeFuture<ProofResponse?> {
    return getProver(proofIndex).findProofResponse(proofIndex)
  }

  override fun isProofAlreadyDone(proofIndex: TProofIndex): SafeFuture<Boolean> {
    return getProver(proofIndex).isProofAlreadyDone(proofIndex)
  }

  override fun requestProof(proofRequest: ProofRequest): SafeFuture<ProofResponse> {
    return getProver(proofRequest).requestProof(proofRequest)
  }

  override fun getProofIndex(proofRequest: ProofRequest): TProofIndex =
    getProver(proofRequest).getProofIndex(proofRequest)

  override fun createProofRequest(proofRequest: ProofRequest): SafeFuture<TProofIndex> {
    return getProver(proofRequest).createProofRequest(proofRequest)
  }

  override fun removeRequests(startBlockNumberGte: Long?): SafeFuture<Unit> {
    return (proverA?.removeRequests(startBlockNumberGte) ?: SafeFuture.completedFuture(Unit))
      .thenCompose {
        proverB?.removeRequests(startBlockNumberGte) ?: SafeFuture.completedFuture(Unit)
      }
  }
}
