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
 * For each conflation, [GlobalBlobCalculator.newConflation] calls, in order:
 * 1. [checkOverflow] with the conflation, before it is appended: a trigger flushes the open blob without it;
 * 2. the conflation is appended to the stream, which may seal full blobs ([reset] runs for each);
 * 3. [appendConflation] with the conflation;
 * 4. [checkTrigger] with the conflation: a trigger flushes the open blob with it as the last conflation.
 *
 * Triggers may only depend on the conflations' blocks (numbers, timestamps), not on how a conflation was cut: after a
 * restart, conflations are rebuilt from persisted batches.
 */
interface BlobTriggerCalculator {
  /** Records [conflation] as part of the open blob. */
  fun appendConflation(conflation: BlocksConflation)

  /**
   * Checked after [conflation] is appended. A trigger ends the open blob right AFTER [conflation], which becomes its
   * last conflation. For conditions met by a conflation itself, e.g. one ending at a target block number.
   */
  fun checkTrigger(conflation: BlocksConflation): BlobTrigger?

  /**
   * Checked before [next] is appended. A trigger ends the open blob BEFORE [next], which then starts a new blob. For
   * boundaries that [next] would cross, e.g. [next] being the first conflation after a hard fork.
   */
  fun checkOverflow(next: BlocksConflation): BlobTrigger? = null

  /** Called when a blob is sealed (a new blob opens). */
  fun reset()
}

/** A conflation together with the result of [linea.da.DaCompressor.compress] for it. */
data class CompressedBlocksConflation(
  val conflation: BlocksConflation,
  val compressed: CompressedConflation,
)

/** A compressed conflation together with where its segment ends in a sealed blob. */
data class SealedConflation(
  val conflation: CompressedBlocksConflation,
  val cut: SegmentEnd,
)

/**
 * One sealed blob and every conflation whose segment ends in it, in order.
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
