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
): BlobData =
  BlobData(
    chunkHash = chunkHash,
    blobBytes = blobBytes,
    prevDataRollingHash = prevDataRollingHash,
    dataRollingHash = dataRollingHash,
  )

/**
 * Creates a record whose blobs chain by rolling hash. By default it has one fresh blob from [parentDataRollingHash] to
 * [dataRollingHash]; with [parentSharedBlobCarryOverTailBytes] > 0 that blob is the shared one, from
 * [parentBlobDataRollingHash] to [parentDataRollingHash], and a second blob follows it up to [dataRollingHash].
 */
fun createRiscvRollupRecord(
  startBlockNumber: ULong,
  endBlockNumber: ULong,
  startBlockTimestamp: Instant = Clock.System.now().trimToSecondPrecision(),
  endBlockTimestamp: Instant? = null,
  parentDataRollingHash: ByteArray = Random.nextBytes(32),
  dataRollingHash: ByteArray = Random.nextBytes(32),
  totalBatchesCount: UInt = 1U,
  parentSharedBlobCarryOverTailBytes: UInt = 0u,
  parentBlobDataRollingHash: ByteArray? = if (parentSharedBlobCarryOverTailBytes > 0u) Random.nextBytes(32) else null,
  finalSharedBlobCarryOverTailBytes: UInt = 0u,
  blobsData: List<BlobData> = if (parentBlobDataRollingHash != null) {
    listOf(
      createBlobData(prevDataRollingHash = parentBlobDataRollingHash, dataRollingHash = parentDataRollingHash),
      createBlobData(prevDataRollingHash = parentDataRollingHash, dataRollingHash = dataRollingHash),
    )
  } else {
    listOf(createBlobData(prevDataRollingHash = parentDataRollingHash, dataRollingHash = dataRollingHash))
  },
  proofHash: ByteArray = Random.nextBytes(32),
): RiscvRollupRecord {
  val resolvedEndBlockTimestamp = endBlockTimestamp
    ?: startBlockTimestamp
      .plus(LINEA_BLOCK_INTERVAL.times((endBlockNumber - startBlockNumber).toInt()))
      .trimToSecondPrecision()
  return RiscvRollupRecord(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    startBlockTimestamp = startBlockTimestamp,
    endBlockTimestamp = resolvedEndBlockTimestamp,
    parentDataRollingHash = parentDataRollingHash,
    dataRollingHash = dataRollingHash,
    totalBatchesCount = totalBatchesCount,
    blobsData = blobsData,
    proofHash = proofHash,
    parentSharedBlobCarryOverTailBytes = parentSharedBlobCarryOverTailBytes,
    parentBlobDataRollingHash = parentBlobDataRollingHash,
    finalSharedBlobCarryOverTailBytes = finalSharedBlobCarryOverTailBytes,
  )
}
