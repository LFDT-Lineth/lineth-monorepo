package lineth.conflation.calculators

import linea.domain.BlocksConflation

class RollupTriggerCalculatorByConflationCount(
  private val maxConflations: Int,
) : RollupTriggerCalculator {
  private var count = 0

  override fun appendConflation(conflation: BlocksConflation) {
    count++
  }

  override fun checkTrigger(conflation: BlocksConflation): RollupTrigger? =
    if (count >= maxConflations) RollupTrigger.CONFLATION_COUNT else null

  override fun reset() {
    count = 0
  }
}
