package net.consensys.zkevm.coordinator.clients

import linea.domain.BlockInterval
import linea.domain.BlockIntervals
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.ethereum.coordination.blob.ShnarfResult

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
  val kzgProofSideCar: ByteArray
) : BlockInterval {
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
    return result
  }
}

// It only needs to parse a subset of the data to send to L1 or populate the DB.
data class BlobCompressionProof(
  val compressedData: ByteArray, // The data that are explicitly sent in the blob (i.e. after compression)
  val conflationOrder: BlockIntervals,
  val prevShnarf: ByteArray,
  val parentStateRootHash: ByteArray, // Parent root hash
  val finalStateRootHash: ByteArray, // New state root hash
  val parentDataHash: ByteArray,
  val dataHash: ByteArray,
  val snarkHash: ByteArray, // The snarkHash used for the blob consistency check
  val expectedX: ByteArray,
  val expectedY: ByteArray,
  val expectedShnarf: ByteArray,
  val decompressionProof: ByteArray, // zkProof of compression and consistency
  val proverVersion: String,
  val verifierID: Long,
  val commitment: ByteArray,
  val kzgProofContract: ByteArray,
  val kzgProofSidecar: ByteArray
) {

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobCompressionProof

    if (!compressedData.contentEquals(other.compressedData)) return false
    if (conflationOrder != other.conflationOrder) return false
    if (!prevShnarf.contentEquals(other.prevShnarf)) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (!finalStateRootHash.contentEquals(other.finalStateRootHash)) return false
    if (!parentDataHash.contentEquals(other.parentDataHash)) return false
    if (!dataHash.contentEquals(other.dataHash)) return false
    if (!snarkHash.contentEquals(other.snarkHash)) return false
    if (!expectedX.contentEquals(other.expectedX)) return false
    if (!expectedY.contentEquals(other.expectedY)) return false
    if (!expectedShnarf.contentEquals(other.expectedShnarf)) return false
    if (!decompressionProof.contentEquals(other.decompressionProof)) return false
    if (proverVersion != other.proverVersion) return false
    if (verifierID != other.verifierID) return false
    if (!commitment.contentEquals(other.commitment)) return false
    if (!kzgProofContract.contentEquals(other.kzgProofContract)) return false
    if (!kzgProofSidecar.contentEquals(other.kzgProofSidecar)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = compressedData.contentHashCode()
    result = 31 * result + conflationOrder.hashCode()
    result = 31 * result + prevShnarf.contentHashCode()
    result = 31 * result + parentStateRootHash.contentHashCode()
    result = 31 * result + finalStateRootHash.contentHashCode()
    result = 31 * result + parentDataHash.contentHashCode()
    result = 31 * result + dataHash.contentHashCode()
    result = 31 * result + snarkHash.contentHashCode()
    result = 31 * result + expectedX.contentHashCode()
    result = 31 * result + expectedY.contentHashCode()
    result = 31 * result + expectedShnarf.contentHashCode()
    result = 31 * result + decompressionProof.contentHashCode()
    result = 31 * result + proverVersion.hashCode()
    result = 31 * result + verifierID.hashCode()
    result = 31 * result + commitment.contentHashCode()
    result = 31 * result + kzgProofContract.contentHashCode()
    result = 31 * result + kzgProofSidecar.contentHashCode()

    return result
  }
}
