package lineth.conflation.calculators

import linea.da.CompressedConflation
import linea.da.SealedBlob
import linea.da.SegmentEnd
import linea.domain.BlocksConflation

/** Why a blob was flushed. Values are named and ordered as in [linea.domain.ConflationTrigger]. */
enum class BlobTrigger {
  TARGET_BLOCK_NUMBER,
  HARD_FORK,
  TIME_LIMIT,
}

/**
 * Decides when the open blob is flushed (ended early), in the same shape as [RollupTriggerCalculator].
 *
 * Triggers may only depend on the conflations' blocks (numbers, timestamps), not on how a conflation was cut: after a
 * restart, conflations are rebuilt from persisted batches.
 */
interface BlobTriggerCalculator {
  fun appendConflation(conflation: BlocksConflation)

  /** Flush right AFTER [conflation] (inclusive). */
  fun checkTrigger(conflation: BlocksConflation): BlobTrigger?

  /** Flush BEFORE [next] is appended, e.g. when [next] is the first conflation after a hard fork. */
  fun checkOverflow(next: BlocksConflation): BlobTrigger? = null

  /** Called when a blob is sealed (a new blob opens). */
  fun reset()
}

/** A conflation together with the result of [linea.da.DaStream.compress] for it. */
data class CompressedBlocksConflation(
  val conflation: BlocksConflation,
  val compressed: CompressedConflation,
)

/** A compressed conflation together with where its frame ends in a sealed blob. */
data class SealedConflation(
  val conflation: CompressedBlocksConflation,
  val cut: SegmentEnd,
)

/**
 * One sealed blob and every conflation whose frame ends in it, in order.
 *
 * - A conflation spanning several blobs appears once, in the event of the blob where it ends; the blobs it fills
 *   entirely produce events with no conflations.
 * - [next] is the conflation after the last cut, known thanks to lazy sealing; null after a flush.
 * - [flushed] is true when the blob was ended early. A flushed blob always has at least one conflation.
 */
data class BlobEvent(
  val blob: SealedBlob,
  val conflations: List<SealedConflation>,
  val next: CompressedBlocksConflation?,
  val flushed: Boolean,
)

/**
 * Receives each sealed blob. Called synchronously while [GlobalBlobCalculator] is mid-update: must not call back into
 * the calculators (`newConflation`, `flush`, `checkDeadline`), which would emit events out of order.
 */
fun interface BlobHandler {
  fun onBlob(event: BlobEvent)
}
