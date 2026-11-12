package lineth.conflation.calculators

import linea.domain.BlocksConflation

enum class RollupTrigger {
  CONFLATION_COUNT,
  TARGET_BLOCK_NUMBER,
  DATA_LIMIT,
  GAS,
  TX_COUNT,
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
