package linea.domain

import kotlin.time.Instant

data class BlobCompressionProofRequest(
  val compressedData: ByteArray,
  val conflations: List<ConflationCalculationResult>,
  val parentStateRootHash: ByteArray,
  val finalStateRootHash: ByteArray,
  val parentDataHash: ByteArray,
  val prevShnarf: ByteArray,
  val expectedShnarfResult: ShnarfResult,
  val commitment: ByteArray,
  val kzgProofContract: ByteArray,
  val kzgProofSideCar: ByteArray,
  override val startBlockTimestamp: Instant,
) : BlockInterval, StartBlockTimestampProvider {
  override val startBlockNumber: ULong
    get() = conflations.first().startBlockNumber
  override val endBlockNumber: ULong
    get() = conflations.last().endBlockNumber

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobCompressionProofRequest

    if (!compressedData.contentEquals(other.compressedData)) return false
    if (conflations != other.conflations) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (!finalStateRootHash.contentEquals(other.finalStateRootHash)) return false
    if (!parentDataHash.contentEquals(other.parentDataHash)) return false
    if (!prevShnarf.contentEquals(other.prevShnarf)) return false
    if (expectedShnarfResult != other.expectedShnarfResult) return false
    if (!commitment.contentEquals(other.commitment)) return false
    if (!kzgProofContract.contentEquals(other.kzgProofContract)) return false
    if (!kzgProofSideCar.contentEquals(other.kzgProofSideCar)) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false

    return true
  }

  override fun hashCode(): Int {
    var result = compressedData.contentHashCode()
    result = 31 * result + conflations.hashCode()
    result = 31 * result + parentStateRootHash.contentHashCode()
    result = 31 * result + finalStateRootHash.contentHashCode()
    result = 31 * result + parentDataHash.contentHashCode()
    result = 31 * result + prevShnarf.contentHashCode()
    result = 31 * result + expectedShnarfResult.hashCode()
    result = 31 * result + commitment.contentHashCode()
    result = 31 * result + kzgProofContract.contentHashCode()
    result = 31 * result + kzgProofSideCar.contentHashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    return result
  }
}
