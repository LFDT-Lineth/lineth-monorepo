package linea.da

import linea.clients.RollupProofRequestV1
import linea.kotlin.encodeHex

/**
 * Initial state of a [DaStream]: the end of the last proven rollup.
 *
 * With [tailBytes] = 0 the stream starts on a fresh blob after [dataRollingHash]. With [tailBytes] > 0 the last rollup
 * ended inside [sharedBlob], whose last [tailBytes] payload bytes belong to the next rollup. The stream then resumes
 * inside that blob, and the following appends must reproduce those bytes exactly.
 *
 * @property dataRollingHash rolling hash after the last rollup's last blob
 * @property tailBytes the last rollup's `finalDataTailDiscardBytes`; 0 = fresh blob
 * @property sharedBlob the last rollup's last (packed) blob, required when [tailBytes] > 0
 * @property prevDataRollingHash rolling hash before [sharedBlob], required when [tailBytes] > 0
 */
data class DaStreamPosition(
  val dataRollingHash: ByteArray,
  val tailBytes: Int = 0,
  val sharedBlob: ByteArray? = null,
  val prevDataRollingHash: ByteArray? = null,
) {
  init {
    require(dataRollingHash.size == 32) { "dataRollingHash must be 32 bytes, got ${dataRollingHash.size}" }
    require(tailBytes in 0..RollupProofRequestV1.MAX_TAIL_BYTES) {
      "tailBytes=$tailBytes must be in 0..${RollupProofRequestV1.MAX_TAIL_BYTES}"
    }
    if (tailBytes > 0) {
      require(sharedBlob?.size == DaConstants.BLOB_BYTES) {
        "sharedBlob must be ${DaConstants.BLOB_BYTES} bytes when tailBytes > 0, got ${sharedBlob?.size}"
      }
      require(prevDataRollingHash?.size == 32) {
        "prevDataRollingHash must be 32 bytes when tailBytes > 0, got ${prevDataRollingHash?.size}"
      }
    } else {
      require(sharedBlob == null && prevDataRollingHash == null) {
        "sharedBlob and prevDataRollingHash must be null when tailBytes = 0"
      }
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as DaStreamPosition

    if (!dataRollingHash.contentEquals(other.dataRollingHash)) return false
    if (tailBytes != other.tailBytes) return false
    if (!sharedBlob.contentEquals(other.sharedBlob)) return false
    if (!prevDataRollingHash.contentEquals(other.prevDataRollingHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = dataRollingHash.contentHashCode()
    result = 31 * result + tailBytes
    result = 31 * result + (sharedBlob?.contentHashCode() ?: 0)
    result = 31 * result + (prevDataRollingHash?.contentHashCode() ?: 0)
    return result
  }

  override fun toString(): String {
    return "DaStreamPosition(dataRollingHash=${dataRollingHash.encodeHex()}, tailBytes=$tailBytes, " +
      "sharedBlob.size=${sharedBlob?.size}, prevDataRollingHash=${prevDataRollingHash?.encodeHex()})"
  }
}
