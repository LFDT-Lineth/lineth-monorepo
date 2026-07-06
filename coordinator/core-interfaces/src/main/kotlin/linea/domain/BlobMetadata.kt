package linea.domain

import kotlin.time.Instant

/**
 * Blob DA-submission metadata: everything needed to submit a blob to L1 and to chain shnarfs,
 * stored as a versioned JSON envelope ([BlobMetadata]) in the `blobs.blob_submission_metadata`
 * column. Replaces the embedded `BlobCompressionProof` — the new flow has no standalone
 * compression proof.
 *
 * Timestamps are `kotlin.time.Instant` (serialized as epoch millis by the codec); `batchesCount`
 * is `Int` so the JSON stays primitive.
 */
sealed interface BlobMetadataPayload : VersionedMetadataPayload

typealias BlobMetadata = VersionedMetadata<BlobMetadataPayload>

enum class BlobMetadataVersion(val value: Int, val payloadClass: Class<out BlobMetadataPayload>) {
  V1(1, BlobMetadataV1::class.java),
  ;

  companion object {
    fun fromValue(value: Int): BlobMetadataVersion =
      entries.firstOrNull { it.value == value }
        ?: throw IllegalArgumentException("Unknown BlobMetadata version: $value")
  }
}

data class BlobMetadataV1(
  // denormalized identity/query fields (self-contained snapshot)
  val startBlockTimestamp: Instant,
  val endBlockTimestamp: Instant,
  val batchesCount: Int,
  // DA payload
  val blobHash: ByteArray,
  val expectedShnarf: ByteArray,
  val prevShnarf: ByteArray,
  val finalStateRootHash: ByteArray,
  val compressedData: ByteArray,
  val commitment: ByteArray,
  val kzgProofContract: ByteArray,
  val kzgProofSideCar: ByteArray,
  val expectedX: ByteArray,
  val expectedY: ByteArray,
  val snarkHash: ByteArray,
) : BlobMetadataPayload {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobMetadataV1

    if (startBlockTimestamp != other.startBlockTimestamp) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (batchesCount != other.batchesCount) return false
    if (!blobHash.contentEquals(other.blobHash)) return false
    if (!expectedShnarf.contentEquals(other.expectedShnarf)) return false
    if (!prevShnarf.contentEquals(other.prevShnarf)) return false
    if (!finalStateRootHash.contentEquals(other.finalStateRootHash)) return false
    if (!compressedData.contentEquals(other.compressedData)) return false
    if (!commitment.contentEquals(other.commitment)) return false
    if (!kzgProofContract.contentEquals(other.kzgProofContract)) return false
    if (!kzgProofSideCar.contentEquals(other.kzgProofSideCar)) return false
    if (!expectedX.contentEquals(other.expectedX)) return false
    if (!expectedY.contentEquals(other.expectedY)) return false
    if (!snarkHash.contentEquals(other.snarkHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockTimestamp.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + batchesCount
    result = 31 * result + blobHash.contentHashCode()
    result = 31 * result + expectedShnarf.contentHashCode()
    result = 31 * result + prevShnarf.contentHashCode()
    result = 31 * result + finalStateRootHash.contentHashCode()
    result = 31 * result + compressedData.contentHashCode()
    result = 31 * result + commitment.contentHashCode()
    result = 31 * result + kzgProofContract.contentHashCode()
    result = 31 * result + kzgProofSideCar.contentHashCode()
    result = 31 * result + expectedX.contentHashCode()
    result = 31 * result + expectedY.contentHashCode()
    result = 31 * result + snarkHash.contentHashCode()
    return result
  }
}
