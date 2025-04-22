package linea.domain

import linea.kotlin.encodeHex

data class EthLog(
  val removed: Boolean,
  val logIndex: ULong,
  val transactionIndex: ULong,
  val transactionHash: ByteArray,
  val blockHash: ByteArray,
  val blockNumber: ULong,
  val address: ByteArray,
  val data: ByteArray,
  val topics: List<ByteArray>
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as EthLog

    if (removed != other.removed) return false
    if (logIndex != other.logIndex) return false
    if (transactionIndex != other.transactionIndex) return false
    if (!transactionHash.contentEquals(other.transactionHash)) return false
    if (!blockHash.contentEquals(other.blockHash)) return false
    if (blockNumber != other.blockNumber) return false
    if (!address.contentEquals(other.address)) return false
    if (!data.contentEquals(other.data)) return false
    if (topics.size != other.topics.size || !topics.zip(other.topics).all { (a, b) -> a.contentEquals(b) }) return false

    return true
  }

  override fun hashCode(): Int {
    var result = removed.hashCode()
    result = 31 * result + logIndex.hashCode()
    result = 31 * result + transactionIndex.hashCode()
    result = 31 * result + transactionHash.contentHashCode()
    result = 31 * result + blockHash.contentHashCode()
    result = 31 * result + blockNumber.hashCode()
    result = 31 * result + address.contentHashCode()
    result = 31 * result + data.contentHashCode()
    result = 31 * result + topics.fold(1) { acc, topic -> 31 * acc + topic.contentHashCode() }
    return result
  }

  override fun toString(): String {
    return "EthLog(" +
      "removed=$removed, " +
      "logIndex=$logIndex, " +
      "transactionIndex=$transactionIndex, " +
      "transactionHash=${transactionHash.encodeHex()}, " +
      "blockHash=${blockHash.encodeHex()}, " +
      "blockNumber=$blockNumber, " +
      "address=${address.encodeHex()}, " +
      "topics=${topics.map { it.encodeHex() }}, " +
      "data=${data.encodeHex()}"
  }
}

data class EthLogEvent<E>(
  val event: E,
  val log: EthLog
) : Comparable<EthLogEvent<E>> {
  override fun compareTo(other: EthLogEvent<E>): Int {
    return when {
      this.log.blockNumber != other.log.blockNumber -> this.log.blockNumber.compareTo(other.log.blockNumber)
      this.log.logIndex != other.log.logIndex -> this.log.logIndex.compareTo(other.log.logIndex)
      else -> 0
    }
  }
}
