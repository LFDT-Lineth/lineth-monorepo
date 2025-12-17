package net.consensys.zkevm.domain

import kotlinx.datetime.Instant
import linea.domain.BlockInterval
import linea.domain.BlockIntervals
import linea.domain.CommonDomainFunctions
import net.consensys.zkevm.coordinator.clients.BlobCompressionProof

data class Blob(
  val conflations: List<ConflationCalculationResult>,
  val compressedData: ByteArray,
  val startBlockTime: Instant,
  val endBlockTime: Instant,
) : BlockInterval {
  override val startBlockNumber: ULong
    get() = conflations.first().startBlockNumber
  override val endBlockNumber: ULong
    get() = conflations.last().endBlockNumber

  private fun isAllConsecutive(conflations: List<ConflationCalculationResult>): Boolean {
    return conflations.foldIndexed(true) { i, acc, next ->
      acc && (i == 0 || next.startBlockNumber == conflations[i - 1].endBlockNumber + 1UL)
    }
  }

  init {
    require(isAllConsecutive(conflations)) {
      "Conflations are not consecutive: ${conflations.map { it.intervalString() }}"
    }
    require(endBlockTime >= startBlockTime) {
      "End block time predates start block time: endBlockTime=$endBlockTime vs startBlockTime=$startBlockTime"
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as Blob

    if (!compressedData.contentEquals(other.compressedData)) return false
    if (conflations != other.conflations) return false
    if (startBlockTime != other.startBlockTime) return false
    if (endBlockTime != other.endBlockTime) return false

    return true
  }

  override fun hashCode(): Int {
    var result = compressedData.contentHashCode()
    result = 31 * result + conflations.hashCode()
    result = 31 * result + startBlockTime.hashCode()
    result = 31 * result + endBlockTime.hashCode()
    return result
  }

  override val blocksRange: ULongRange = conflations.first().startBlockNumber..conflations.last().endBlockNumber

  override fun toString(): String {
    return "Blob(" +
      "blocksRange=${CommonDomainFunctions.blockIntervalString(blocksRange.first, blocksRange.last)}" +
      "startBlockTime=$startBlockTime " +
      "endBlockTime=$endBlockTime " +
      "conflations=$conflations " +
      "compressedData=${compressedData.size} bytes " +
      ")"
  }
}

data class BlobAndBatchCounters(
  val blobCounters: BlobCounters,
  val executionProofs: BlockIntervals,
)

data class BlobCounters(
  val numberOfBatches: UInt,
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val startBlockTimestamp: Instant,
  val endBlockTimestamp: Instant,
  val expectedShnarf: ByteArray,
) : BlockInterval {
  companion object {
    fun areAllConsecutive(blobs: List<BlobCounters>): Boolean {
      return blobs.foldIndexed(true) { i, acc, next ->
        acc && (i == 0 || next.startBlockNumber == blobs[i - 1].endBlockNumber + 1UL)
      }
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobCounters

    if (numberOfBatches != other.numberOfBatches) return false
    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (!expectedShnarf.contentEquals(other.expectedShnarf)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = numberOfBatches.hashCode()
    result = 31 * result + startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + expectedShnarf.contentHashCode()
    return result
  }
}

data class BlobRecord(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val blobHash: ByteArray,
  val startBlockTime: Instant,
  val endBlockTime: Instant,
  val batchesCount: UInt,
  val expectedShnarf: ByteArray,
  // Unproven records will have null here
  val blobCompressionProof: BlobCompressionProof? = null,
) : BlockInterval {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobRecord

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!blobHash.contentEquals(other.blobHash)) return false
    if (startBlockTime != other.startBlockTime) return false
    if (endBlockTime != other.endBlockTime) return false
    if (batchesCount != other.batchesCount) return false
    if (!expectedShnarf.contentEquals(expectedShnarf)) return false
    if (blobCompressionProof != other.blobCompressionProof) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blobHash.contentHashCode()
    result = 31 * result + startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTime.hashCode()
    result = 31 * result + endBlockTime.hashCode()
    result = 31 * result + batchesCount.hashCode()
    result = 31 * result + expectedShnarf.contentHashCode()
    result = 31 * result + blobCompressionProof.hashCode()

    return result
  }
}

enum class BlobStatus {
  COMPRESSION_PROVING,
  COMPRESSION_PROVEN,
}

data class BlobSubmittedEvent(
  val blobs: List<BlockInterval>,
  val endBlockTime: Instant,
  val lastShnarf: ByteArray,
  val submissionTimestamp: Instant,
  val transactionHash: ByteArray,
) {
  fun getSubmissionDelay(): Long {
    return submissionTimestamp.minus(endBlockTime).inWholeSeconds
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobSubmittedEvent

    if (blobs != other.blobs) return false
    if (endBlockTime != other.endBlockTime) return false
    if (!lastShnarf.contentEquals(other.lastShnarf)) return false
    if (submissionTimestamp != other.submissionTimestamp) return false
    if (transactionHash.contentEquals(transactionHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blobs.hashCode()
    result = 31 * result + endBlockTime.hashCode()
    result = 31 * result + lastShnarf.contentHashCode()
    result = 31 * result + submissionTimestamp.hashCode()
    result = 31 * result + transactionHash.contentHashCode()
    return result
  }
}
