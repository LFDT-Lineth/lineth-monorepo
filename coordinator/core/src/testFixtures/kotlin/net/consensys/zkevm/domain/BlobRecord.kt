package net.consensys.zkevm.domain

import build.linea.domain.BlockIntervals
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import net.consensys.linea.blob.ShnarfCalculatorVersion
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.setFirstByteToZero
import net.consensys.trimToSecondPrecision
import net.consensys.zkevm.coordinator.clients.BlobCompressionProof
import net.consensys.zkevm.domain.Constants.LINEA_BLOCK_INTERVAL
import net.consensys.zkevm.ethereum.coordination.blob.BlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobShnarfCalculator
import kotlin.random.Random

private val meterRegistry = SimpleMeterRegistry()
private val metricsFacade: MetricsFacade =
  MicrometerMetricsFacade(registry = meterRegistry, metricsPrefix = "linea")
private val shnarfCalculator: BlobShnarfCalculator =
  GoBackedBlobShnarfCalculator(version = ShnarfCalculatorVersion.V0_1_0, metricsFacade = metricsFacade)

fun createBlobRecord(
  startBlockNumber: ULong? = null,
  endBlockNumber: ULong? = null,
  compressedData: ByteArray = Random.nextBytes(32).setFirstByteToZero(),
  blobHash: ByteArray? = null,
  parentStateRootHash: ByteArray? = Random.nextBytes(32).setFirstByteToZero(),
  parentShnarf: ByteArray? = Random.nextBytes(32),
  parentDataHash: ByteArray? = Random.nextBytes(32),
  shnarf: ByteArray? = null,
  eip4844Enabled: Boolean = true,
  startBlockTime: Instant? = null,
  batchesCount: UInt = 1U,
  parentBlobRecord: BlobRecord? = null,
  blobCompressionProof: BlobCompressionProof? = null
): BlobRecord {
  require(
    blobCompressionProof != null ||
      (startBlockNumber != null && endBlockNumber != null)
  ) { "Either blobCompressionProof or startBlockNumber and endBlockNumber must be provided" }
  val _startBlockNumber = startBlockNumber ?: blobCompressionProof!!.conflationOrder.startingBlockNumber
  val _endBlockNumber = endBlockNumber ?: blobCompressionProof!!.conflationOrder.upperBoundaries.last()
  val _startBlockTime = startBlockTime?.trimToSecondPrecision() ?: Clock.System.now().trimToSecondPrecision()
  val endBlockTime = _startBlockTime
    .plus(LINEA_BLOCK_INTERVAL.times((_endBlockNumber - _startBlockNumber).toInt()))
    .trimToSecondPrecision()
  val finalStateRootHash = Random.nextBytes(32).setFirstByteToZero()
  val _parentStateRootHash = (parentStateRootHash ?: parentBlobRecord?.blobCompressionProof?.finalStateRootHash)!!
  val _prevShnarf = (parentShnarf ?: parentBlobRecord?.blobCompressionProof?.expectedShnarf)!!
  val shnarfResult = shnarfCalculator.calculateShnarf(
    compressedData = compressedData,
    parentStateRootHash = _parentStateRootHash,
    finalStateRootHash = finalStateRootHash,
    prevShnarf = _prevShnarf,
    conflationOrder = BlockIntervals(_startBlockNumber, listOf(_endBlockNumber))
  )
  val _dataHash = blobHash ?: shnarfResult.dataHash
  val _parentDataHash = parentDataHash ?: parentBlobRecord?.blobCompressionProof?.dataHash ?: Random.nextBytes(32)
  val _blobCompressionProof = blobCompressionProof ?: BlobCompressionProof(
    compressedData = compressedData,
    conflationOrder = BlockIntervals(_startBlockNumber, listOf(_endBlockNumber)),
    prevShnarf = _prevShnarf,
    parentStateRootHash = _parentStateRootHash,
    finalStateRootHash = finalStateRootHash,
    parentDataHash = _parentDataHash,
    dataHash = _dataHash,
    snarkHash = shnarfResult.snarkHash,
    expectedX = shnarfResult.expectedX,
    expectedY = shnarfResult.expectedY,
    expectedShnarf = shnarf ?: shnarfResult.expectedShnarf,
    decompressionProof = Random.nextBytes(512),
    proverVersion = "mock-0.0.0",
    verifierID = 6789,
    commitment = if (eip4844Enabled) Random.nextBytes(48) else ByteArray(0),
    kzgProofContract = if (eip4844Enabled) Random.nextBytes(48) else ByteArray(0),
    kzgProofSidecar = if (eip4844Enabled) Random.nextBytes(48) else ByteArray(0)
  )
  return BlobRecord(
    startBlockNumber = _startBlockNumber,
    endBlockNumber = _endBlockNumber,
    blobHash = _blobCompressionProof.dataHash,
    startBlockTime = _startBlockTime,
    endBlockTime = endBlockTime,
    batchesCount = batchesCount,
    expectedShnarf = _blobCompressionProof.expectedShnarf,
    blobCompressionProof = _blobCompressionProof
  )
}

fun createBlobRecords(
  blobsIntervals: BlockIntervals,
  parentDataHash: ByteArray = Random.nextBytes(32),
  parentShnarf: ByteArray = Random.nextBytes(32),
  parentStateRootHash: ByteArray = Random.nextBytes(32)
): List<BlobRecord> {
  val firstBlob = createBlobRecord(
    startBlockNumber = blobsIntervals.startingBlockNumber,
    endBlockNumber = blobsIntervals.upperBoundaries.first(),
    parentDataHash = parentDataHash,
    parentShnarf = parentShnarf,
    parentStateRootHash = parentStateRootHash
  )

  return blobsIntervals
    .toIntervalList()
    .drop(1)
    .fold(mutableListOf(firstBlob)) { acc, interval ->
      val blob = createBlobRecord(
        startBlockNumber = interval.startBlockNumber,
        endBlockNumber = interval.endBlockNumber,
        parentBlobRecord = acc.last()
      )
      acc.add(blob)
      acc
    }
}

fun createBlobRecords(
  compressionProofs: List<BlobCompressionProof>,
  firstBlockStartBlockTime: Instant = Clock.System.now().trimToSecondPrecision()
): List<BlobRecord> {
  require(compressionProofs.isNotEmpty()) { "At least one compression proof must be provided" }
  val sortedCompressionProofs = compressionProofs.sortedBy { it.conflationOrder.startingBlockNumber }

  val firstBlob = createBlobRecord(
    startBlockTime = firstBlockStartBlockTime,
    blobCompressionProof = sortedCompressionProofs.first()
  )

  return sortedCompressionProofs
    .drop(1)
    .fold(mutableListOf(firstBlob)) { acc, proof ->
      val parentBlobRecord = acc.last()
      val blob = createBlobRecord(
        startBlockTime = parentBlobRecord.endBlockTime.plus(LINEA_BLOCK_INTERVAL),
        parentBlobRecord = parentBlobRecord,
        blobCompressionProof = proof
      )
      acc.add(blob)
      acc
    }
}

fun createBlobRecordFromBatches(
  batches: List<Batch>,
  blobCompressionProof: BlobCompressionProof? = null
): BlobRecord {
  val startBlockNumber = batches.first().startBlockNumber
  val endBlockNumber = batches.last().endBlockNumber
  val startBlockTime: Instant = Clock.System.now().trimToSecondPrecision()
  val endBlockTime = startBlockTime
    .plus(LINEA_BLOCK_INTERVAL.times((endBlockNumber - startBlockNumber).toInt()))
    .trimToSecondPrecision()

  return BlobRecord(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    blobHash = Random.nextBytes(32).setFirstByteToZero(),
    startBlockTime = startBlockTime,
    endBlockTime = endBlockTime,
    batchesCount = batches.size.toUInt(),
    expectedShnarf = Random.nextBytes(32),
    blobCompressionProof = blobCompressionProof
  )
}
