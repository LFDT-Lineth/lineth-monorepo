package linea.domain

data class StreamPosition(
  val lastConflationEndBlock: ULong,
  val dataRollingHash: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false
    other as StreamPosition
    if (lastConflationEndBlock != other.lastConflationEndBlock) return false
    if (!dataRollingHash.contentEquals(other.dataRollingHash)) return false
    return true
  }

  override fun hashCode(): Int {
    var result = lastConflationEndBlock.hashCode()
    result = 31 * result + dataRollingHash.contentHashCode()
    return result
  }
}

/**
 * Folds a chunk hash into the data rolling hash: keccak256(parentDataRollingHash || chunkHash).
 */
fun interface DataRollingHashCalculator {
  fun fold(parentDataRollingHash: ByteArray, chunkHash: ByteArray): ByteArray
}
