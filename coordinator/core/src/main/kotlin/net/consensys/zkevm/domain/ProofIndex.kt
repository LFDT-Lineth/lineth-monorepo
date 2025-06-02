package net.consensys.zkevm.domain

import linea.domain.BlockInterval

data class ProofIndex(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val hash: ByteArray? = null,
) : BlockInterval {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ProofIndex

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (hash != null) {
      if (other.hash == null) return false
      if (!hash.contentEquals(other.hash)) return false
    } else if (other.hash != null) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + (hash?.contentHashCode() ?: 0)
    return result
  }
}
