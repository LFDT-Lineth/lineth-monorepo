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
  parentDataRollingHash: ByteArray = Random.nextBytes(32),
  dataRollingHash: ByteArray = Random.nextBytes(32),
  totalConflationsCount: UInt = 1U,
  blobsData: List<BlobData> = listOf(
    BlobData(
      chunkHash = Random.nextBytes(32),
      blobBytes = Random.nextBytes(32),
      conflationsCount = 1U,
    ),
  ),
  proofHash: ByteArray = Random.nextBytes(32),
  endOffset: Int = Constants.Eip4844BlobSize,
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
    parentDataRollingHash = parentDataRollingHash,
    dataRollingHash = dataRollingHash,
    totalConflationsCount = totalConflationsCount,
    blobsData = blobsData,
    proofHash = proofHash,
    endOffset = endOffset,
  )
}
