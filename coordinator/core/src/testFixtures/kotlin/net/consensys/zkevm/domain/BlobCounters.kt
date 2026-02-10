package net.consensys.zkevm.domain

import net.consensys.zkevm.domain.Constants.LINEA_BLOCK_INTERVAL
import kotlin.random.Random
import kotlin.time.Clock
import kotlin.time.Instant

fun blobCounters(
  startBlockNumber: ULong,
  endBlockNumber: ULong,
  numberOfBatches: UInt = 2u,
  startBlockTimestamp: Instant = Clock.System.now(),
  endBlockTimestamp: Instant = startBlockTimestamp.plus(LINEA_BLOCK_INTERVAL * numberOfBatches.toInt()),
  expectedShnarf: ByteArray = Random.nextBytes(32),
): BlobCounters {
  return BlobCounters(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    numberOfBatches = numberOfBatches,
    startBlockTimestamp = startBlockTimestamp,
    endBlockTimestamp = endBlockTimestamp,
    expectedShnarf = expectedShnarf,
  )
}
