package lineth.conflation.calculators

import linea.domain.BlocksConflation

/** Ends the rollup right after a conflation ending at a target block number (inclusive). */
class RollupTriggerCalculatorByTargetBlockNumbers(
  private val targetEndBlockNumbers: Set<ULong>,
) : RollupTriggerCalculator {
  override fun appendConflation(conflation: BlocksConflation) = Unit

  override fun checkTrigger(conflation: BlocksConflation): RollupTrigger? =
    if (conflation.endBlockNumber in targetEndBlockNumbers) RollupTrigger.TARGET_BLOCK_NUMBER else null

  override fun reset() = Unit
}

/**
 * Base for the overflow triggers: sums a per-conflation measure over the open rollup, and ends the rollup BEFORE the
 * next conflation if adding it would exceed [limit]. A single conflation larger than [limit] still forms a rollup on
 * its own, because the check only runs once the rollup has a conflation.
 */
abstract class RollupTriggerCalculatorBySum(
  private val limit: ULong,
  private val trigger: RollupTrigger,
) : RollupTriggerCalculator {
  private var sum: ULong = 0UL

  protected abstract fun measure(conflation: CompressedBlocksConflation): ULong

  override fun appendConflation(conflation: BlocksConflation) =
    throw UnsupportedOperationException("${this::class.simpleName} needs the compressed conflation")

  override fun appendConflation(conflation: CompressedBlocksConflation) {
    sum += measure(conflation)
  }

  override fun checkTrigger(conflation: BlocksConflation): RollupTrigger? = null

  override fun checkOverflow(next: CompressedBlocksConflation): RollupTrigger? =
    if (sum + measure(next) > limit) trigger else null

  override fun reset() {
    sum = 0UL
  }
}

/** Ends the rollup before the next conflation if the rollup's DA bytes (compressed frames) would exceed [limit]. */
class RollupTriggerCalculatorByDataLimit(limit: ULong) : RollupTriggerCalculatorBySum(limit, RollupTrigger.DATA_LIMIT) {
  override fun measure(conflation: CompressedBlocksConflation): ULong =
    conflation.compressed.compressedBytes.toULong()
}

/** Ends the rollup before the next conflation if the rollup's gas used would exceed [limit]. */
class RollupTriggerCalculatorByGas(limit: ULong) : RollupTriggerCalculatorBySum(limit, RollupTrigger.GAS) {
  override fun measure(conflation: CompressedBlocksConflation): ULong =
    conflation.conflation.blocks.sumOf { it.gasUsed }
}

/** Ends the rollup before the next conflation if the rollup's transaction count would exceed [limit]. */
class RollupTriggerCalculatorByTxCount(limit: ULong) : RollupTriggerCalculatorBySum(limit, RollupTrigger.TX_COUNT) {
  override fun measure(conflation: CompressedBlocksConflation): ULong =
    conflation.conflation.blocks.sumOf { it.transactions.size.toULong() }
}
