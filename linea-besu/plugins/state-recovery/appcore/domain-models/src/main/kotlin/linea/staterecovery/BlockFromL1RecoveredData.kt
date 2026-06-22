package linea.staterecovery

import linea.kotlin.encodeHex
import kotlin.time.Instant

data class BlockHeaderFromL1RecoveredData(
  val blockNumber: ULong,
  val blockHash: ByteArray,
  val coinbase: ByteArray,
  val blockTimestamp: Instant,
  val gasLimit: ULong,
  val difficulty: ULong,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlockHeaderFromL1RecoveredData

    if (blockNumber != other.blockNumber) return false
    if (!blockHash.contentEquals(other.blockHash)) return false
    if (!coinbase.contentEquals(other.coinbase)) return false
    if (blockTimestamp != other.blockTimestamp) return false
    if (gasLimit != other.gasLimit) return false
    if (difficulty != other.difficulty) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blockNumber.hashCode()
    result = 31 * result + blockHash.contentHashCode()
    result = 31 * result + coinbase.contentHashCode()
    result = 31 * result + blockTimestamp.hashCode()
    result = 31 * result + gasLimit.hashCode()
    result = 31 * result + difficulty.hashCode()
    return result
  }

  override fun toString(): String {
    return "BlockHeaderFromL1RecoveredData(" +
      "blockNumber=$blockNumber, " +
      "blockHash=${blockHash.encodeHex()}, " +
      "coinbase=${coinbase.encodeHex()}," +
      "blockTimestamp=$blockTimestamp, " +
      "gasLimit=$gasLimit, " +
      "difficulty=$difficulty" +
      ")"
  }
}

data class BlockFromL1RecoveredData(
  val header: BlockHeaderFromL1RecoveredData,
  val transactions: List<TransactionFromL1RecoveredData>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlockFromL1RecoveredData

    if (header != other.header) return false
    if (transactions != other.transactions) return false

    return true
  }

  override fun hashCode(): Int {
    var result = header.hashCode()
    result = 31 * result + transactions.hashCode()
    return result
  }

  override fun toString(): String {
    return "BlockFromL1RecoveredData(header=$header, transactions=$transactions)"
  }
}
