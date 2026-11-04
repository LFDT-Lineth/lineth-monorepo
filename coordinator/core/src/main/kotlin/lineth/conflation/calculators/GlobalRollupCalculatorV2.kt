package lineth.conflation.calculators

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.time.Duration
import kotlin.time.Instant

/**
 * Applies rollup triggers at each cut point of the [BlobEvent]s from
 * [GlobalBlobCalculator] and emits [Rollup]s.
 *
 * At each conflation ending in a sealed blob, the rollup ends there if:
 * - the blob was flushed and this is its last cut (a flushed blob ends the rollup), or
 * - an inclusive trigger fires for that conflation (conflation count, target block number), or
 * - an overflow trigger says adding the next conflation would exceed a limit (data, gas, tx count).
 *
 * Not thread-safe: confined to the thread that drives [GlobalBlobCalculator].
 *
 * @param blobCalculator the blob calculator feeding this one, used to flush the open blob. Passed as a provider
 *   because the blob calculator in turn takes this calculator as its [BlobHandler].
 * @param rollupDeadline ends the open rollup (by flushing the open blob) once its oldest pending conflation's first
 *   block is this old; see [checkDeadline].
 */
class GlobalRollupCalculatorV2(
  private val triggers: List<RollupTriggerCalculator>,
  private val openRollup: RollupDaAssembler,
  private val blobCalculator: () -> GlobalBlobCalculator,
  private val rollupDeadline: Duration? = null,
  private val log: Logger = LogManager.getLogger(GlobalRollupCalculatorV2::class.java),
) : RollupCalculatorV2 {
  private var rollupHandler: RollupHandlerV2? = null

  override fun onRollup(handler: RollupHandlerV2) {
    rollupHandler = handler
  }

  override fun onBlob(event: BlobEvent) {
    openRollup.addBlob(event.blob)
    event.conflations.forEachIndexed { i, sealed ->
      val current = sealed.conflation
      openRollup.addConflation(current.conflation)
      triggers.forEach { it.appendConflation(current) }
      val next = event.conflations.getOrNull(i + 1)?.conflation ?: event.next
      val endHere = (event.flushed && i == event.conflations.lastIndex) ||
        triggers.any { it.checkTrigger(current.conflation) != null } ||
        (next != null && triggers.any { it.checkOverflow(next) != null })
      if (endHere) {
        val rollup = openRollup.endRollup(sealed.cut)
        triggers.forEach { it.reset() }
        log.debug(
          "rollup cut: blocks={}..{} blobs={} tailTake={} tailDiscard={}",
          rollup.startBlockNumber,
          rollup.endBlockNumber,
          rollup.da.blobs.size,
          rollup.da.parentDataTailTakeBytes,
          rollup.da.expectedFinalDataTailDiscardBytes,
        )
        rollupHandler?.onRollup(rollup)
      }
    }
  }

  /**
   * Rollup deadline: once the oldest conflation not yet in a rollup has a first block at least [rollupDeadline] old at
   * [now], flushes the open blob. The flushed blob then ends the rollup at its last cut, including the conflations
   * still in the open blob. The oldest pending conflation is the open rollup's first one, or, when the open rollup has
   * none yet, the oldest conflation still in the open blob.
   *
   * @return true if a blob was flushed.
   */
  fun checkDeadline(now: Instant): Boolean {
    val deadline = rollupDeadline ?: return false
    val blobs = blobCalculator()
    val oldest = openRollup.openConflations.firstOrNull() ?: blobs.oldestUnsealedConflation ?: return false
    val oldestBlockTimestamp = Instant.fromEpochSeconds(oldest.blocks.first().timestamp.toLong())
    return if (now - oldestBlockTimestamp >= deadline) blobs.flush(BlobTrigger.TIME_LIMIT) else false
  }
}
