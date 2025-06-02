package net.consensys.zkevm.domain

import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import net.consensys.zkevm.domain.Constants.LINEA_BLOCK_INTERVAL
import kotlin.random.Random

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
