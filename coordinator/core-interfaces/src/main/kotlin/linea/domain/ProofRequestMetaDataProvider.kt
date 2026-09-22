package linea.domain

import kotlin.time.Instant

/**
 * Aggregated metadata over the block range covered by a proof request, propagated from execution proofs
 * up through rollup and rollup-aggregation proofs via [BlockIntervalProofIndex.proofRequestMetaData].
 */
interface ProofRequestMetaDataProvider {
  val endBlockTimestamp: Instant?
  val transactionsCount: Long?
  val totalGasUsed: Long?
}

/**
 * Holds the [ProofRequestMetaDataProvider] fields as a single nullable value on [BlockIntervalProofIndex], so
 * callers that don't care about this metadata (e.g. tests building an index just to identify a block interval)
 * don't need to supply it. Each field is itself nullable/defaulted so a partially known [ProofRequestMetaData]
 * can be supplied without having to fill in every field.
 */
data class ProofRequestMetaData(
  val endBlockTimestamp: Instant? = null,
  val transactionsCount: Long? = null,
  val totalGasUsed: Long? = null,
)

/**
 * Sums [selector] over each [BlockIntervalProofIndex]'s [BlockIntervalProofIndex.proofRequestMetaData], returning
 * null (rather than treating a missing value as zero) if any entry's [ProofRequestMetaData] or selected field is
 * unknown, since a partial sum would misrepresent the aggregate.
 */
fun List<BlockIntervalProofIndex>.sumOfMetaDataOrNull(selector: (ProofRequestMetaData) -> Long?): Long? {
  var sum = 0L
  for (proofIndex in this) {
    val value = proofIndex.proofRequestMetaData?.let(selector) ?: return null
    sum += value
  }
  return sum
}
