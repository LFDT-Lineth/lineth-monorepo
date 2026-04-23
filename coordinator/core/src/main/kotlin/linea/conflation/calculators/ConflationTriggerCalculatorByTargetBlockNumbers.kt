package linea.conflation.calculators

import linea.domain.BlockCounters
import linea.domain.ConflationTrigger

class ConflationTriggerCalculatorByTargetBlockNumbers(
  private val targetEndBlockNumbers: Set<ULong>,
  override val id: String = ConflationTrigger.TARGET_BLOCK_NUMBER.name,
) : ConflationTriggerCalculator {
  override fun checkOverflow(blockCounters: BlockCounters): ConflationTriggerCalculator.OverflowTrigger? {
    return if (targetEndBlockNumbers.contains(blockCounters.blockNumber - 1uL)) {
      ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.TARGET_BLOCK_NUMBER, false)
    } else {
      null
    }
  }

  override fun appendBlock(blockCounters: BlockCounters) {}

  override fun reset() {}

  override fun copyCountersTo(counters: ConflationCounters) {}
}
