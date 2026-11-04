package linea.domain

import linea.domain.TestConstants.LINEA_BLOCK_INTERVAL
import linea.kotlin.trimToSecondPrecision
import kotlin.random.Random
import kotlin.time.Clock
import kotlin.time.Instant

fun createBlobData(
  prevDataRollingHash: ByteArray = Random.nextBytes(32),
  dataRollingHash: ByteArray = Random.nextBytes(32),
  blobBytes: ByteArray = Random.nextBytes(32),
  chunkHash: ByteArray = Random.nextBytes(32),
  batchesCount: UInt = 1U,
): BlobData =
  BlobData(
    chunkHash = chunkHash,
    blobBytes = blobBytes,
    batchesCount = batchesCount,
    prevDataRollingHash = prevDataRollingHash,
    dataRollingHash = dataRollingHash,
  )

/**
 * Creates a record whose blobs chain by rolling hash. By default it has one fresh blob from [parentDataRollingHash] to
 * [dataRollingHash]; with [parentDataTailTakeBytes] > 0 that blob is the shared one, from
 * [boundaryPrevDataRollingHash] to [parentDataRollingHash], and a second blob follows it up to [dataRollingHash].
 */
fun createBlobRecordV2(
  startBlockNumber: ULong,
  endBlockNumber: ULong,
  startBlockTimestamp: Instant = Clock.System.now().trimToSecondPrecision(),
  endBlockTimestamp: Instant? = null,
  parentDataRollingHash: ByteArray = Random.nextBytes(32),
  dataRollingHash: ByteArray = Random.nextBytes(32),
  totalBatchesCount: UInt = 1U,
  parentDataTailTakeBytes: Int = 0,
  boundaryPrevDataRollingHash: ByteArray? = if (parentDataTailTakeBytes > 0) Random.nextBytes(32) else null,
  finalDataTailDiscardBytes: Int = 0,
  blobsData: List<BlobData> = if (boundaryPrevDataRollingHash != null) {
    listOf(
      createBlobData(prevDataRollingHash = boundaryPrevDataRollingHash, dataRollingHash = parentDataRollingHash),
      createBlobData(prevDataRollingHash = parentDataRollingHash, dataRollingHash = dataRollingHash),
    )
  } else {
    listOf(createBlobData(prevDataRollingHash = parentDataRollingHash, dataRollingHash = dataRollingHash))
  },
  proofHash: ByteArray = Random.nextBytes(32),
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
    totalBatchesCount = totalBatchesCount,
    blobsData = blobsData,
    proofHash = proofHash,
    parentDataTailTakeBytes = parentDataTailTakeBytes,
    boundaryPrevDataRollingHash = boundaryPrevDataRollingHash,
    finalDataTailDiscardBytes = finalDataTailDiscardBytes,
  )
}
