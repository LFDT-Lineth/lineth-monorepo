package linea.domain

/*
 * All hashes, shnarf, x, y values have 32 bytes
 */
data class ShnarfResult(
  val dataHash: ByteArray,
  val snarkHash: ByteArray,
  val expectedX: ByteArray,
  val expectedY: ByteArray,
  val expectedShnarf: ByteArray,
  val commitment: ByteArray,
  val kzgProofContract: ByteArray,
  val kzgProofSideCar: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ShnarfResult

    if (!dataHash.contentEquals(other.dataHash)) return false
    if (!snarkHash.contentEquals(other.snarkHash)) return false
    if (!expectedX.contentEquals(other.expectedX)) return false
    if (!expectedY.contentEquals(other.expectedY)) return false
    if (!expectedShnarf.contentEquals(other.expectedShnarf)) return false
    if (!commitment.contentEquals(other.commitment)) return false
    if (!kzgProofContract.contentEquals(other.kzgProofContract)) return false
    if (!kzgProofSideCar.contentEquals(other.kzgProofSideCar)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = dataHash.contentHashCode()
    result = 31 * result + snarkHash.contentHashCode()
    result = 31 * result + expectedX.contentHashCode()
    result = 31 * result + expectedY.contentHashCode()
    result = 31 * result + expectedShnarf.contentHashCode()
    result = 31 * result + commitment.contentHashCode()
    result = 31 * result + kzgProofContract.contentHashCode()
    result = 31 * result + kzgProofSideCar.contentHashCode()
    return result
  }
}

interface BlobShnarfCalculator {
  fun calculateShnarf(
    compressedData: ByteArray,
    parentStateRootHash: ByteArray,
    finalStateRootHash: ByteArray,
    prevShnarf: ByteArray,
    conflationOrder: BlockIntervals,
  ): ShnarfResult
}
