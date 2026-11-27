package lineth.persistence

import linea.domain.RiscvRollupRecord
import linea.error.DuplicatedRecordException
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentSkipListMap
import kotlin.time.Instant

/**
 * In-memory [RiscvRollupsDao] mirroring the semantics of `RiscvRollupsPostgresDao`. Rollups are keyed by start block
 * number; start and end block numbers are each unique, so a rollup sharing only one of them with a stored rollup fails.
 * Blobs are held by the records themselves, so a blob shared by adjacent rollups needs no separate bookkeeping.
 *
 * @param maxRollupProofsToReturn cap on [getConsecutiveRollupsFromBlockNumber], applied before the timestamp filter
 * @param ignoreDuplicates when true, saving a stored rollup again succeeds without changes; when false it fails with
 *   [DuplicatedRecordException]
 */
class FakeRiscvRollupsDao(
  val rollups: ConcurrentSkipListMap<ULong, RiscvRollupRecord> = ConcurrentSkipListMap(),
  private val maxRollupProofsToReturn: UInt = UInt.MAX_VALUE,
  private val ignoreDuplicates: Boolean = true,
) : RiscvRollupsDao {

  @Synchronized
  override fun saveRollupWithBlobs(rollup: RiscvRollupRecord): SafeFuture<Unit> {
    val sameStart = rollups[rollup.startBlockNumber]
    if (sameStart != null && sameStart.endBlockNumber == rollup.endBlockNumber) {
      return if (ignoreDuplicates) {
        SafeFuture.completedFuture(Unit)
      } else {
        SafeFuture.failedFuture(DuplicatedRecordException("Rollup ${rollup.intervalString()} is already persisted!"))
      }
    }
    if (sameStart != null || rollups.values.any { it.endBlockNumber == rollup.endBlockNumber }) {
      return SafeFuture.failedFuture(
        IllegalStateException("Rollup ${rollup.intervalString()} shares its start or end block with a stored rollup"),
      )
    }
    rollups[rollup.startBlockNumber] = rollup
    return SafeFuture.completedFuture(Unit)
  }

  override fun getConsecutiveRollupsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<RiscvRollupRecord>> =
    SafeFuture.completedFuture(
      consecutiveRun(startingBlockNumberInclusive)
        .take(maxRollupProofsToReturn.coerceAtMost(Int.MAX_VALUE.toUInt()).toInt())
        .filter { it.endBlockTimestamp < endBlockCreatedBefore }
        .toList(),
    )

  override fun findHighestConsecutiveRollupFromBlockNumber(
    startingBlockNumberInclusive: ULong,
  ): SafeFuture<RiscvRollupRecord?> =
    SafeFuture.completedFuture(consecutiveRun(startingBlockNumberInclusive).lastOrNull())

  override fun findRollupByStartBlockNumber(startBlockNumber: ULong): SafeFuture<RiscvRollupRecord?> =
    SafeFuture.completedFuture(rollups[startBlockNumber])

  override fun findRollupByEndBlockNumber(endBlockNumber: ULong): SafeFuture<RiscvRollupRecord?> =
    SafeFuture.completedFuture(rollups.values.find { it.endBlockNumber == endBlockNumber })

  @Synchronized
  override fun deleteRollupsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> =
    SafeFuture.completedFuture(removeIf { it.endBlockNumber <= endBlockNumberInclusive })

  @Synchronized
  override fun deleteRollupsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> =
    SafeFuture.completedFuture(removeIf { it.startBlockNumber >= startingBlockNumberInclusive })

  private fun consecutiveRun(startingBlockNumberInclusive: ULong): Sequence<RiscvRollupRecord> =
    generateSequence(rollups[startingBlockNumberInclusive]) { rollups[it.endBlockNumber + 1UL] }

  private fun removeIf(predicate: (RiscvRollupRecord) -> Boolean): Int {
    val toRemove = rollups.values.filter(predicate)
    toRemove.forEach { rollups.remove(it.startBlockNumber) }
    return toRemove.size
  }
}
