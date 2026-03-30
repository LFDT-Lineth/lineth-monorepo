package net.consensys.zkevm.ethereum.coordination

import java.util.concurrent.ConcurrentSkipListSet

class DynamicBlockNumberSet(
  initialBlockNumbers: Collection<ULong> = emptyList(),
  private val blockNumbers: ConcurrentSkipListSet<ULong> = ConcurrentSkipListSet<ULong>(initialBlockNumbers),
) : Set<ULong> by blockNumbers {

  fun addBlockNumber(blockNumber: ULong) {
    blockNumbers.add(blockNumber)
  }

  fun addBlockNumbers(blockNumbers: Collection<ULong>) {
    this.blockNumbers.addAll(blockNumbers)
  }

  fun removeBlockNumber(blockNumber: ULong): Boolean {
    return blockNumbers.remove(blockNumber)
  }
}
