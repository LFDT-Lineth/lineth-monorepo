package lineth.da

import linea.kotlin.encodeHex

/**
 * Initial state of a [DaStream]: the end of the last proven rollup.
 *
 * With [sharedBlobCarryOverTailBytes] = 0 the stream starts on a fresh blob after [dataRollingHash]. Otherwise the last
 * rollup ended inside [sharedBlob], whose last [sharedBlobCarryOverTailBytes] payload bytes carry over to the next
 * rollup. The stream then resumes inside that blob, and the following appends must reproduce those bytes exactly.
 *
 * @property dataRollingHash rolling hash after the last rollup's last blob
 * @property parentBlobDataRollingHash rolling hash before the blob the stream opens in: before [sharedBlob] when
 *   [sharedBlobCarryOverTailBytes] > 0, so that `fold(parentBlobDataRollingHash, chunkHash(sharedBlob)) ==
 *   dataRollingHash`; equal to [dataRollingHash] when [sharedBlobCarryOverTailBytes] = 0, as the next blob folds onto it
 * @property sharedBlobCarryOverTailBytes the last rollup's `finalSharedBlobCarryOverTailBytes`; 0 = fresh blob
 * @property sharedBlob the last rollup's last (packed) blob, required when [sharedBlobCarryOverTailBytes] > 0
 */
data class DaStreamPosition(
  val dataRollingHash: ByteArray,
  val parentBlobDataRollingHash: ByteArray,
  val sharedBlobCarryOverTailBytes: UInt = 0u,
  val sharedBlob: ByteArray? = null,
) {
  init {
    require(dataRollingHash.size == 32) { "dataRollingHash must be 32 bytes, got ${dataRollingHash.size}" }
    require(parentBlobDataRollingHash.size == 32) {
      "parentBlobDataRollingHash must be 32 bytes, got ${parentBlobDataRollingHash.size}"
    }
    require(sharedBlobCarryOverTailBytes <= DaConstants.MAX_TAIL_BYTES) {
      "sharedBlobCarryOverTailBytes=$sharedBlobCarryOverTailBytes must be in 0..${DaConstants.MAX_TAIL_BYTES}"
    }
    if (sharedBlobCarryOverTailBytes > 0u) {
      require(sharedBlob?.size == DaConstants.BLOB_BYTES) {
        "sharedBlob must be ${DaConstants.BLOB_BYTES} bytes when sharedBlobCarryOverTailBytes > 0, " +
          "got ${sharedBlob?.size}"
      }
    } else {
      require(sharedBlob == null) { "sharedBlob must be null when sharedBlobCarryOverTailBytes = 0" }
      require(parentBlobDataRollingHash.contentEquals(dataRollingHash)) {
        "parentBlobDataRollingHash must equal dataRollingHash when sharedBlobCarryOverTailBytes = 0"
      }
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as DaStreamPosition

    if (!dataRollingHash.contentEquals(other.dataRollingHash)) return false
    if (!parentBlobDataRollingHash.contentEquals(other.parentBlobDataRollingHash)) return false
    if (sharedBlobCarryOverTailBytes != other.sharedBlobCarryOverTailBytes) return false
    if (!sharedBlob.contentEquals(other.sharedBlob)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = dataRollingHash.contentHashCode()
    result = 31 * result + parentBlobDataRollingHash.contentHashCode()
    result = 31 * result + sharedBlobCarryOverTailBytes.hashCode()
    result = 31 * result + (sharedBlob?.contentHashCode() ?: 0)
    return result
  }

  override fun toString(): String {
    return "DaStreamPosition(dataRollingHash=${dataRollingHash.encodeHex()}, " +
      "parentBlobDataRollingHash=${parentBlobDataRollingHash.encodeHex()}, " +
      "sharedBlobCarryOverTailBytes=$sharedBlobCarryOverTailBytes, " +
      "sharedBlob.size=${sharedBlob?.size})"
  }

  companion object {
    /** A position on a fresh blob after [dataRollingHash]: at genesis, or after a rollup ending at a blob boundary. */
    fun fresh(dataRollingHash: ByteArray): DaStreamPosition = DaStreamPosition(dataRollingHash, dataRollingHash)
  }
}
