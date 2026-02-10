package net.consensys.zkevm.ethereum.coordination.aggregation

import kotlin.time.Instant

data class AggregationL2State(
  val parentAggregationLastBlockTimestamp: Instant,
  val parentAggregationLastL1RollingHashMessageNumber: ULong,
  val parentAggregationLastL1RollingHash: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as AggregationL2State

    if (parentAggregationLastBlockTimestamp != other.parentAggregationLastBlockTimestamp) return false
    if (parentAggregationLastL1RollingHashMessageNumber != other.parentAggregationLastL1RollingHashMessageNumber) {
      return false
    }
    if (!parentAggregationLastL1RollingHash.contentEquals(other.parentAggregationLastL1RollingHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = parentAggregationLastBlockTimestamp.hashCode()
    result = 31 * result + parentAggregationLastL1RollingHashMessageNumber.hashCode()
    result = 31 * result + parentAggregationLastL1RollingHash.contentHashCode()
    return result
  }
}
