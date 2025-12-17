package net.consensys.zkevm.domain

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.blob.ShnarfCalculatorVersion
import linea.domain.BlockIntervals
import linea.kotlin.setFirstByteToZero
import linea.kotlin.trimToSecondPrecision
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.zkevm.coordinator.clients.BlobCompressionProof
import net.consensys.zkevm.domain.Constants.LINEA_BLOCK_INTERVAL
import net.consensys.zkevm.ethereum.coordination.blob.BlobShnarfCalculator
import net.consensys.zkevm.ethereum.coordination.blob.GoBackedBlobShnarfCalculator
import kotlin.random.Random

private val meterRegistry = SimpleMeterRegistry()
private val metricsFacade: MetricsFacade =
  MicrometerMetricsFacade(registry = meterRegistry, metricsPrefix = "linea")
private val shnarfCalculator: BlobShnarfCalculator =
  GoBackedBlobShnarfCalculator(version = ShnarfCalculatorVersion.V1_2, metricsFacade = metricsFacade)

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
  blobCompressionProof: BlobCompressionProof? = null,
): BlobRecord {
  require(
    blobCompressionProof != null ||
      (startBlockNumber != null && endBlockNumber != null),
  ) { "Either blobCompressionProof or startBlockNumber and endBlockNumber must be provided" }
  val resolvedStartBlockNumber = startBlockNumber ?: blobCompressionProof!!.conflationOrder.startingBlockNumber
  val resolvedEndBlockNumber = endBlockNumber ?: blobCompressionProof!!.conflationOrder.upperBoundaries.last()
  val resolvedStartBlockTime = startBlockTime?.trimToSecondPrecision() ?: Clock.System.now().trimToSecondPrecision()
  val endBlockTime = resolvedStartBlockTime
    .plus(LINEA_BLOCK_INTERVAL.times((resolvedEndBlockNumber - resolvedStartBlockNumber).toInt()))
    .trimToSecondPrecision()
  val finalStateRootHash = Random.nextBytes(32).setFirstByteToZero()
  val resolvedParentStateRootHash = (
    parentStateRootHash
      ?: parentBlobRecord?.blobCompressionProof?.finalStateRootHash
    )!!
  val resolvedPrevShnarf = (parentShnarf ?: parentBlobRecord?.blobCompressionProof?.expectedShnarf)!!
  val shnarfResult = shnarfCalculator.calculateShnarf(
    compressedData = compressedData,
    parentStateRootHash = resolvedParentStateRootHash,
    finalStateRootHash = finalStateRootHash,
    prevShnarf = resolvedPrevShnarf,
    conflationOrder = BlockIntervals(resolvedStartBlockNumber, listOf(resolvedEndBlockNumber)),
  )
  val resolvedDataHash = blobHash ?: shnarfResult.dataHash
  val resolvedParentDataHash =
    parentDataHash ?: parentBlobRecord?.blobCompressionProof?.dataHash ?: Random.nextBytes(32)
  val resolvedBlobCompressionProof = blobCompressionProof ?: BlobCompressionProof(
    compressedData = compressedData,
    conflationOrder = BlockIntervals(resolvedStartBlockNumber, listOf(resolvedEndBlockNumber)),
    prevShnarf = resolvedPrevShnarf,
    parentStateRootHash = resolvedParentStateRootHash,
    finalStateRootHash = finalStateRootHash,
    parentDataHash = resolvedParentDataHash,
    dataHash = resolvedDataHash,
    snarkHash = shnarfResult.snarkHash,
    expectedX = shnarfResult.expectedX,
    expectedY = shnarfResult.expectedY,
    expectedShnarf = shnarf ?: shnarfResult.expectedShnarf,
    decompressionProof = Random.nextBytes(512),
    proverVersion = "mock-0.0.0",
    verifierID = 6789,
    commitment = if (eip4844Enabled) Random.nextBytes(48) else ByteArray(0),
    kzgProofContract = if (eip4844Enabled) Random.nextBytes(48) else ByteArray(0),
    kzgProofSidecar = if (eip4844Enabled) Random.nextBytes(48) else ByteArray(0),
  )
  return BlobRecord(
    startBlockNumber = resolvedStartBlockNumber,
    endBlockNumber = resolvedEndBlockNumber,
    blobHash = resolvedBlobCompressionProof.dataHash,
    startBlockTime = resolvedStartBlockTime,
    endBlockTime = endBlockTime,
    batchesCount = batchesCount,
    expectedShnarf = resolvedBlobCompressionProof.expectedShnarf,
    blobCompressionProof = resolvedBlobCompressionProof,
  )
}

fun createBlobRecords(
  blobsIntervals: BlockIntervals,
  parentDataHash: ByteArray = Random.nextBytes(32),
  parentShnarf: ByteArray = Random.nextBytes(32),
  parentStateRootHash: ByteArray = Random.nextBytes(32),
): List<BlobRecord> {
  val firstBlob = createBlobRecord(
    startBlockNumber = blobsIntervals.startingBlockNumber,
    endBlockNumber = blobsIntervals.upperBoundaries.first(),
    parentDataHash = parentDataHash,
    parentShnarf = parentShnarf,
    parentStateRootHash = parentStateRootHash,
  )

  return blobsIntervals
    .toIntervalList()
    .drop(1)
    .fold(mutableListOf(firstBlob)) { acc, interval ->
      val blob = createBlobRecord(
        startBlockNumber = interval.startBlockNumber,
        endBlockNumber = interval.endBlockNumber,
        parentBlobRecord = acc.last(),
      )
      acc.add(blob)
      acc
    }
}

fun createBlobRecords(
  compressionProofs: List<BlobCompressionProof>,
  firstBlockStartBlockTime: Instant = Clock.System.now().trimToSecondPrecision(),
): List<BlobRecord> {
  require(compressionProofs.isNotEmpty()) { "At least one compression proof must be provided" }
  val sortedCompressionProofs = compressionProofs.sortedBy { it.conflationOrder.startingBlockNumber }

  val firstBlob = createBlobRecord(
    startBlockTime = firstBlockStartBlockTime,
    blobCompressionProof = sortedCompressionProofs.first(),
  )

  return sortedCompressionProofs
    .drop(1)
    .fold(mutableListOf(firstBlob)) { acc, proof ->
      val parentBlobRecord = acc.last()
      val blob = createBlobRecord(
        startBlockTime = parentBlobRecord.endBlockTime.plus(LINEA_BLOCK_INTERVAL),
        parentBlobRecord = parentBlobRecord,
        blobCompressionProof = proof,
      )
      acc.add(blob)
      acc
    }
}

fun createBlobRecordFromBatches(batches: List<Batch>, blobCompressionProof: BlobCompressionProof? = null): BlobRecord {
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
    blobCompressionProof = blobCompressionProof,
  )
}
