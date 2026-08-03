package linea.domain

import linea.domain.TestConstants.LINEA_BLOCK_INTERVAL
import linea.kotlin.trimToSecondPrecision
import kotlin.random.Random
import kotlin.time.Clock
import kotlin.time.Instant

fun createBlobRecordV2(
  startBlockNumber: ULong,
  endBlockNumber: ULong,
  startBlockTimestamp: Instant = Clock.System.now().trimToSecondPrecision(),
  endBlockTimestamp: Instant? = null,
  parentShnarf: ByteArray = Random.nextBytes(32),
  endShnarf: ByteArray = Random.nextBytes(32),
  totalBatchesCount: UInt = 1U,
  blobsData: List<BlobData> = listOf(
    BlobData(
      blobHash = Random.nextBytes(32),
      compressedData = Random.nextBytes(32),
      batchesCount = 1U,
    ),
  ),
): BlobRecordV2 {
  val resolvedEndBlockTimestamp = endBlockTimestamp
    ?: startBlockTimestamp
      .plus(LINEA_BLOCK_INTERVAL.times((endBlockNumber - startBlockNumber).toInt()))
      .trimToSecondPrecision()
  return BlobRecordV2(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    startBlockTimestamp = startBlockTimestamp,
    endBlockTimestamp = resolvedEndBlockTimestamp,
    parentShnarf = parentShnarf,
    endShnarf = endShnarf,
    totalBatchesCount = totalBatchesCount,
    blobsData = blobsData,
  )
}
