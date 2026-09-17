package linea.domain

data class StreamPosition(
  val lastConflationEndBlock: ULong,
  val dataRollingHash: ByteArray,
  val endOffset: Int,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false
    other as StreamPosition
    if (lastConflationEndBlock != other.lastConflationEndBlock) return false
    if (!dataRollingHash.contentEquals(other.dataRollingHash)) return false
    if (endOffset != other.endOffset) return false
    return true
  }

  override fun hashCode(): Int {
    var result = lastConflationEndBlock.hashCode()
    result = 31 * result + dataRollingHash.contentHashCode()
    result = 31 * result + endOffset
    return result
  }
}

object DataRollingHashCalculator {
  fun fold(parentDataRollingHash: ByteArray, chunkHash: ByteArray): ByteArray =
    TODO("implement: keccak256(parentDataRollingHash + chunkHash)")
}
