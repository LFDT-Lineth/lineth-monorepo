package linea.conflation.calculators

import linea.domain.BlockCounters
import linea.domain.ConflationTrigger

class ConflationTriggerCalculatorByBlockLimit(
  private val blockLimit: UInt,
) : ConflationTriggerCalculator {
  override val id: String = ConflationTrigger.BLOCKS_LIMIT.name
  private var blockCount: UInt = 0u

  override fun checkOverflow(blockCounters: BlockCounters): ConflationTriggerCalculator.OverflowTrigger? {
    if (blockCount < blockLimit) {
      return null
    } else {
      return ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.BLOCKS_LIMIT, false)
    }
  }

  override fun appendBlock(blockCounters: BlockCounters) {
    if (blockCount + 1u > blockLimit) {
      throw IllegalStateException("Block ${blockCounters.blockNumber} cannot be appended to current conflation")
    }
    blockCount++
  }

  override fun reset() {
    blockCount = 0u
  }

  override fun copyCountersTo(counters: ConflationCounters) {
    counters.blockCount = blockCount
  }
}
