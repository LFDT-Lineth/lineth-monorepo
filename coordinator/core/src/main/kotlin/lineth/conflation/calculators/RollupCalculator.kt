package lineth.conflation.calculators

import linea.domain.BlocksConflation

/**
 * Why a rollup ended. A rollup is the block range of one rollup proof: one or more whole conflations, whose
 * l2-execution proofs the rollup proof aggregates.
 *
 * Limits are checked before a conflation is added and end the rollup before it, so the rollup stays within the limit;
 * the other triggers end the rollup right after the conflation that hits them. A rollup always holds at least one
 * conflation, so a single conflation over a limit still forms its own rollup.
 */
enum class RollupTrigger {
  /** The rollup holds the configured number of conflations, bounding the l2-execution proofs one rollup proof folds. */
  CONFLATION_COUNT,

  /** The rollup ends at a configured block number, e.g. a planned upgrade or checkpoint that must end a rollup. */
  TARGET_BLOCK_NUMBER,

  /**
   * Adding the next conflation would exceed the DA bytes a rollup proof may parse: the conflations' segments, length
   * prefixes included. Bounds the rollup guest's blob decompression and parsing work.
   */
  DATA_LIMIT,

  /** Adding the next conflation would exceed the gas a rollup may hold, bounding the work it attests to. */
  GAS_LIMIT,

  /** Adding the next conflation would exceed the transactions a rollup may hold, bounding the work it attests to. */
  TX_LIMIT,

  /** The rollup has been open longer than the rollup deadline, counted from its first block's timestamp. */
  TIME_LIMIT,
}

interface RollupTriggerCalculator {
  fun appendConflation(conflation: BlocksConflation)

  /** End the rollup AFTER [conflation] (inclusive). */
  fun checkTrigger(conflation: BlocksConflation): RollupTrigger?

  fun reset()

  /** End the rollup BEFORE [next] if adding it would exceed a limit. */
  fun checkOverflow(next: CompressedBlocksConflation): RollupTrigger? = null

  fun appendConflation(conflation: CompressedBlocksConflation) = appendConflation(conflation.conflation)
}

/**
 * Receives each rollup cut by a [RollupCalculator]. Called synchronously from inside [GlobalBlobCalculator]: must only
 * enqueue the rollup and return, not call back into the calculators.
 */
fun interface RollupHandler {
  fun onRollup(rollup: Rollup)
}

/** Cuts RISC-V rollups out of the [BlobEvent]s emitted by [GlobalBlobCalculator]. */
interface RollupCalculator : BlobHandler {
  fun onRollup(handler: RollupHandler)
}
