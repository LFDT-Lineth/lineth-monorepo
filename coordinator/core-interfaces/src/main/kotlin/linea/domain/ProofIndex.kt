package linea.domain

import linea.kotlin.encodeHex
import kotlin.time.Instant

interface ProofIndex : StartBlockTimestampProvider

data class ExecutionProofIndex(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
  override val startBlockTimestamp: Instant,
) : ProofIndex

data class CompressionProofIndex(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
  val hash: ByteArray,
  override val startBlockTimestamp: Instant,
) : ProofIndex {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as CompressionProofIndex

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!hash.contentEquals(other.hash)) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + hash.contentHashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    return result
  }

  override fun toString(): String {
    return "CompressionProofIndex(" +
      "startBlockNumber=$startBlockNumber, " +
      "endBlockNumber=$endBlockNumber, " +
      "hash=${hash.encodeHex()}, " +
      "startBlockTimestamp=$startBlockTimestamp)"
  }
}

data class AggregationProofIndex(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
  val hash: ByteArray,
  override val startBlockTimestamp: Instant,
) : ProofIndex {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as AggregationProofIndex

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!hash.contentEquals(other.hash)) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + hash.contentHashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    return result
  }

  override fun toString(): String {
    return "AggregationProofIndex(" +
      "startBlockNumber=$startBlockNumber, " +
      "endBlockNumber=$endBlockNumber, " +
      "hash=${hash.encodeHex()}, " +
      "startBlockTimestamp=$startBlockTimestamp)"
  }
}

data class InvalidityProofIndex(
  val simulatedExecutionBlockNumber: ULong,
  val ftxNumber: ULong,
  override val startBlockTimestamp: Instant,
) : ProofIndex {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as InvalidityProofIndex

    if (simulatedExecutionBlockNumber != other.simulatedExecutionBlockNumber) return false
    if (ftxNumber != other.ftxNumber) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false

    return true
  }

  override fun hashCode(): Int {
    var result = simulatedExecutionBlockNumber.hashCode()
    result = 31 * result + ftxNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    return result
  }
}
