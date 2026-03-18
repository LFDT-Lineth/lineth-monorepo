package net.consensys.zkevm.ethereum.coordination.conflation

import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger

class ConflationCalculatorByTargetBlockNumbers(
  private val targetEndBlockNumbers: Set<ULong>,
  override val id: String = ConflationTrigger.TARGET_BLOCK_NUMBER.name,
) : ConflationCalculator {
  override fun checkOverflow(blockCounters: BlockCounters): ConflationCalculator.OverflowTrigger? {
    return if (targetEndBlockNumbers.contains(blockCounters.blockNumber - 1uL)) {
      ConflationCalculator.OverflowTrigger(ConflationTrigger.TARGET_BLOCK_NUMBER, false)
    } else {
      null
    }
  }

  override fun appendBlock(blockCounters: BlockCounters) {}

  override fun reset() {}

  override fun copyCountersTo(counters: ConflationCounters) {}
}
