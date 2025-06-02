package net.consensys.zkevm.ethereum.coordination.blob

import tech.pegasys.teku.infrastructure.async.SafeFuture

data class ParentBlobAndZkStateData(
  val parentBlobHash: ByteArray,
  val parentBlobShnarf: ByteArray,
  val parentStateRootHash: ByteArray,
  val finalStateRootHash: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ParentBlobAndZkStateData

    if (!parentBlobHash.contentEquals(other.parentBlobHash)) return false
    return parentBlobShnarf.contentEquals(other.parentBlobShnarf)
  }

  override fun hashCode(): Int {
    var result = parentBlobHash.contentHashCode()
    result = 31 * result + parentBlobShnarf.contentHashCode()
    return result
  }
}

interface ParentBlobDataProvider {
  fun findParentAndZkStateData(blockRange: ULongRange): SafeFuture<ParentBlobAndZkStateData>
}
