package linea.domain

import kotlin.time.Instant

/**
 * Aggregated metadata over the block range covered by a proof request, propagated from execution proofs
 * up through rollup and rollup-aggregation proofs via [BlockIntervalProofIndex].
 */
interface ProofRequestMetaDataProvider {
  val endBlockTimestamp: Instant
  val transactionsCount: Long
  val totalGasUsed: Long
}
