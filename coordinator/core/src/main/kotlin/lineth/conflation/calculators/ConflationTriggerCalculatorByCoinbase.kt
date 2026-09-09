package lineth.conflation.calculators

import linea.domain.BlockCounters
import linea.domain.ConflationTrigger

/**
 * Triggers conflation when the block coinbase (miner address) changes.
 *
 * The RISC-V execution proof uses the first block's miner as the coinbase for the
 * entire conflation, so all blocks in a batch must share the same coinbase.
 */
class ConflationTriggerCalculatorByCoinbase : ConflationTriggerCalculator {
  override val id: String = ConflationTrigger.COINBASE_CHANGE.name
  private var currentBatchCoinbase: String? = null

  override fun checkOverflow(blockCounters: BlockCounters): ConflationTriggerCalculator.OverflowTrigger? {
    val batchCoinbase = currentBatchCoinbase ?: return null
    return if (blockCounters.coinbase != batchCoinbase) {
      ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.COINBASE_CHANGE, false)
    } else {
      null
    }
  }

  override fun appendBlock(blockCounters: BlockCounters) {
    if (currentBatchCoinbase == null) {
      currentBatchCoinbase = blockCounters.coinbase
    } else if (blockCounters.coinbase != currentBatchCoinbase) {
      throw IllegalStateException(
        "Block ${blockCounters.blockNumber} coinbase=${blockCounters.coinbase} " +
          "differs from batch coinbase=$currentBatchCoinbase",
      )
    }
  }

  override fun reset() {
    currentBatchCoinbase = null
  }

  override fun copyCountersTo(counters: ConflationCounters) = Unit
}
