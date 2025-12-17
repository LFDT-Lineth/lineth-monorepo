package net.consensys.zkevm.ethereum.coordination.conflation

import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger

class ConflationCalculatorByBlockLimit(
  private val blockLimit: UInt,
) : ConflationCalculator {
  override val id: String = ConflationTrigger.BLOCKS_LIMIT.name
  private var blockCount: UInt = 0u

  override fun checkOverflow(blockCounters: BlockCounters): ConflationCalculator.OverflowTrigger? {
    if (blockCount < blockLimit) {
      return null
    } else {
      return ConflationCalculator.OverflowTrigger(ConflationTrigger.BLOCKS_LIMIT, false)
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
