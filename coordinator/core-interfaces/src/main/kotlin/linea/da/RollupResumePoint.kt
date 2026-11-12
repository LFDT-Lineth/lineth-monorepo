package linea.da

import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

/**
 * Where the rollup stage resumes: the next rollup starts after [lastConflationEndBlock], and the [DaStream] opens at
 * [daPosition].
 */
data class RollupResumePoint(
  val lastConflationEndBlock: ULong,
  val daPosition: DaStreamPosition,
)

fun interface RollupResumePointProvider {
  /**
   * @param lastFinalizedBlock end block of the last rollup finalized on L1
   * @param lastFinalizedBlockTimestamp timestamp of [lastFinalizedBlock]
   */
  fun getResumePoint(lastFinalizedBlock: ULong, lastFinalizedBlockTimestamp: Instant): SafeFuture<RollupResumePoint>
}
