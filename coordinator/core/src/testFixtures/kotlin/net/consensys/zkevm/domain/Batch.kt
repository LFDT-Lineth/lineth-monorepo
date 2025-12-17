package net.consensys.zkevm.domain

fun createBatch(startBlockNumber: Long, endBlockNumber: Long): Batch {
  return Batch(
    startBlockNumber = startBlockNumber.toULong(),
    endBlockNumber = endBlockNumber.toULong(),
  )
}
