package linea.domain

import kotlin.time.Instant

interface ProofRequestMetaDataProvider {
  val endBlockTimestamp: Instant?
  val transactionsCount: Long?
  val totalGasUsed: Long?
}
