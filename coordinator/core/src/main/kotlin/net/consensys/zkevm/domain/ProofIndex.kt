package net.consensys.zkevm.domain

import linea.kotlin.encodeHex

interface ProofIndex

data class ExecutionProofIndex(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
) : ProofIndex

data class CompressionProofIndex(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
  val hash: ByteArray,
) : ProofIndex {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as CompressionProofIndex

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!hash.contentEquals(other.hash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + hash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "CompressionProofIndex(" +
      "startBlockNumber=$startBlockNumber, " +
      "endBlockNumber=$endBlockNumber, " +
      "hash=${hash.encodeHex()})"
  }
}

data class AggregationProofIndex(
  val startBlockNumber: ULong,
  val endBlockNumber: ULong,
  val hash: ByteArray,
) : ProofIndex {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as AggregationProofIndex

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!hash.contentEquals(other.hash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + hash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "AggregationProofIndex(" +
      "startBlockNumber=$startBlockNumber, " +
      "endBlockNumber=$endBlockNumber, " +
      "hash=${hash.encodeHex()})"
  }
}

data class InvalidityProofIndex(
  val simulatedExecutionBlockNumber: ULong,
  val ftxNumber: ULong,
) : ProofIndex
