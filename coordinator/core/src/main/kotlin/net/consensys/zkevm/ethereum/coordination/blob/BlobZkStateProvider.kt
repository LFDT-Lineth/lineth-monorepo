package net.consensys.zkevm.ethereum.coordination.blob

import tech.pegasys.teku.infrastructure.async.SafeFuture

data class BlobZkState(
  val parentStateRootHash: ByteArray,
  val finalStateRootHash: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobZkState

    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (!finalStateRootHash.contentEquals(other.finalStateRootHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = parentStateRootHash.contentHashCode()
    result = 31 * result + finalStateRootHash.contentHashCode()
    return result
  }
}

interface BlobZkStateProvider {
  fun getBlobZKState(blockRange: ULongRange): SafeFuture<BlobZkState>
}
