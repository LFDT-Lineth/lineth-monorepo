package linea.domain

import linea.da.DaConstants.MAX_TAIL_BYTES
import kotlin.time.Instant

/**
 * A RISC-V rollup proof and the blobs it covers. Adjacent rollups may share a blob: the previous rollup's last blob is
 * then this rollup's first, and appears in both records (it is persisted once).
 *
 * @property parentDataRollingHash rolling hash at the end of the previous rollup
 * @property dataRollingHash rolling hash after this rollup's last blob
 * @property parentSharedBlobCarryOverTailBytes payload bytes at the end of the first (shared) blob that the previous
 *   rollup carried over to this one; 0 when this rollup starts on a fresh blob
 * @property parentBlobDataRollingHash rolling hash before the shared first blob; non-null iff
 *   [parentSharedBlobCarryOverTailBytes] > 0
 * @property finalSharedBlobCarryOverTailBytes payload bytes at the end of the last blob that this rollup carries over
 *   to the next one
 */
data class RiscvRollupRecord(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val startBlockTimestamp: Instant,
  val endBlockTimestamp: Instant,
  val parentDataRollingHash: ByteArray,
  val dataRollingHash: ByteArray,
  val totalBatchesCount: UInt,
  val blobsData: List<BlobData>,
  val proofHash: ByteArray,
  val parentSharedBlobCarryOverTailBytes: UInt,
  val parentBlobDataRollingHash: ByteArray?,
  val finalSharedBlobCarryOverTailBytes: UInt,
) : BlockInterval {
  init {
    require(blobsData.isNotEmpty()) { "blobsData must not be empty: rollup=${intervalString()}" }
    require(parentSharedBlobCarryOverTailBytes <= MAX_TAIL_BYTES) {
      "parentSharedBlobCarryOverTailBytes=$parentSharedBlobCarryOverTailBytes must be in 0..$MAX_TAIL_BYTES: " +
        "rollup=${intervalString()}"
    }
    require(finalSharedBlobCarryOverTailBytes <= MAX_TAIL_BYTES) {
      "finalSharedBlobCarryOverTailBytes=$finalSharedBlobCarryOverTailBytes must be in 0..$MAX_TAIL_BYTES: " +
        "rollup=${intervalString()}"
    }
    require((parentSharedBlobCarryOverTailBytes > 0u) == (parentBlobDataRollingHash != null)) {
      "parentBlobDataRollingHash must be set iff parentSharedBlobCarryOverTailBytes > 0: rollup=${intervalString()}"
    }
    val first = blobsData.first()
    if (parentBlobDataRollingHash != null) {
      require(
        first.prevDataRollingHash.contentEquals(parentBlobDataRollingHash) &&
          first.dataRollingHash.contentEquals(parentDataRollingHash),
      ) {
        "shared first blob must sit between parentBlobDataRollingHash and parentDataRollingHash: " +
          "rollup=${intervalString()}"
      }
    } else {
      require(first.prevDataRollingHash.contentEquals(parentDataRollingHash)) {
        "first blob must follow parentDataRollingHash: rollup=${intervalString()}"
      }
    }
    blobsData.zipWithNext().forEachIndexed { i, (prev, next) ->
      require(next.prevDataRollingHash.contentEquals(prev.dataRollingHash)) {
        "blob ${i + 1} does not follow blob $i by rolling hash: rollup=${intervalString()}"
      }
    }
    require(blobsData.last().dataRollingHash.contentEquals(dataRollingHash)) {
      "last blob must end at dataRollingHash: rollup=${intervalString()}"
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RiscvRollupRecord

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (!dataRollingHash.contentEquals(other.dataRollingHash)) return false
    if (totalBatchesCount != other.totalBatchesCount) return false
    if (blobsData != other.blobsData) return false
    if (!proofHash.contentEquals(other.proofHash)) return false
    if (parentSharedBlobCarryOverTailBytes != other.parentSharedBlobCarryOverTailBytes) return false
    if (!parentBlobDataRollingHash.contentEquals(other.parentBlobDataRollingHash)) return false
    if (finalSharedBlobCarryOverTailBytes != other.finalSharedBlobCarryOverTailBytes) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + dataRollingHash.contentHashCode()
    result = 31 * result + totalBatchesCount.hashCode()
    result = 31 * result + blobsData.hashCode()
    result = 31 * result + proofHash.contentHashCode()
    result = 31 * result + parentSharedBlobCarryOverTailBytes.hashCode()
    result = 31 * result + (parentBlobDataRollingHash?.contentHashCode() ?: 0)
    result = 31 * result + finalSharedBlobCarryOverTailBytes.hashCode()
    return result
  }
}
