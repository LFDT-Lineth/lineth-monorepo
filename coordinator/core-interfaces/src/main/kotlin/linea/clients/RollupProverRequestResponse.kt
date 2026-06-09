package linea.clients

import linea.domain.BlobCompressionProof
import linea.domain.BlockInterval
import linea.domain.StartBlockTimestampProvider
import linea.kotlin.byteArrayListEquals
import kotlin.time.Instant

data class RollupProofRequestV1(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  override val startBlockTimestamp: Instant,
  val blobs: List<BlobCompressionProof>,
  val parentShnarf: ByteArray,
  val endShnarf: ByteArray,
  val l2ExecutionProofs: List<L2ExecutionProofResponse>,
) : BlockInterval, StartBlockTimestampProvider {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupProofRequestV1

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false
    if (blobs != other.blobs) return false
    if (!parentShnarf.contentEquals(other.parentShnarf)) return false
    if (!endShnarf.contentEquals(other.endShnarf)) return false
    if (l2ExecutionProofs != other.l2ExecutionProofs) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    result = 31 * result + blobs.hashCode()
    result = 31 * result + parentShnarf.contentHashCode()
    result = 31 * result + endShnarf.contentHashCode()
    result = 31 * result + l2ExecutionProofs.hashCode()
    return result
  }
}

/**
 * The 14-field PI tuple emitted by a rollup / rollup-aggregation proof (rollup_spec §2.4).
 *
 * Domain twin of `linea.coordinator.clients.prover.riscv.RollupPublicInputsDto`. Kept here (rather than reusing the
 * DTO) because this module is depended upon by the prover-client modules, not the other way around. Where the DTO
 * uses `String` (hex) this uses `ByteArray`, and where the DTO uses `Long` this uses `ULong`.
 */
data class RollupProofPublicInputs(
  val endBlockNumber: ULong,
  val endBlockTimestamp: Instant,
  val l2L1BridgeTransactionTree: ByteArray,
  val parentL1L2BridgeRollingHash: ByteArray,
  val parentL1L2BridgeRollingHashMessageNumber: ULong,
  val endL1L2BridgeRollingHash: ByteArray,
  val endL1L2BridgeRollingHashMessageNumber: ULong,
  val dynamicChainConfigHash: ByteArray,
  val parentFtxRollingHash: ByteArray,
  val endFtxRollingHash: ByteArray,
  val lastProcessedFtxNumber: ULong,
  val filteredAddressesHash: ByteArray,
  val parentShnarf: ByteArray,
  val endShnarf: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupProofPublicInputs

    if (endBlockNumber != other.endBlockNumber) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (!l2L1BridgeTransactionTree.contentEquals(other.l2L1BridgeTransactionTree)) return false
    if (!parentL1L2BridgeRollingHash.contentEquals(other.parentL1L2BridgeRollingHash)) return false
    if (parentL1L2BridgeRollingHashMessageNumber != other.parentL1L2BridgeRollingHashMessageNumber) return false
    if (!endL1L2BridgeRollingHash.contentEquals(other.endL1L2BridgeRollingHash)) return false
    if (endL1L2BridgeRollingHashMessageNumber != other.endL1L2BridgeRollingHashMessageNumber) return false
    if (!dynamicChainConfigHash.contentEquals(other.dynamicChainConfigHash)) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (!endFtxRollingHash.contentEquals(other.endFtxRollingHash)) return false
    if (lastProcessedFtxNumber != other.lastProcessedFtxNumber) return false
    if (!filteredAddressesHash.contentEquals(other.filteredAddressesHash)) return false
    if (!parentShnarf.contentEquals(other.parentShnarf)) return false
    if (!endShnarf.contentEquals(other.endShnarf)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = endBlockNumber.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + l2L1BridgeTransactionTree.contentHashCode()
    result = 31 * result + parentL1L2BridgeRollingHash.contentHashCode()
    result = 31 * result + parentL1L2BridgeRollingHashMessageNumber.hashCode()
    result = 31 * result + endL1L2BridgeRollingHash.contentHashCode()
    result = 31 * result + endL1L2BridgeRollingHashMessageNumber.hashCode()
    result = 31 * result + dynamicChainConfigHash.contentHashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + endFtxRollingHash.contentHashCode()
    result = 31 * result + lastProcessedFtxNumber.hashCode()
    result = 31 * result + filteredAddressesHash.contentHashCode()
    result = 31 * result + parentShnarf.contentHashCode()
    result = 31 * result + endShnarf.contentHashCode()
    return result
  }
}

/**
 * Response of a rollup proof.
 *
 * Mirrors `linea.coordinator.clients.prover.riscv.RollupProofResponseDto`: the DTO's `String` (hex) fields are
 * `ByteArray` here so a proof response — whether read from a JSON file or returned by a REST endpoint — deserializes
 * into the DTO and maps onto this domain type.
 */
data class RollupProofResponse(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val proof: ByteArray,
  val publicInputs: RollupProofPublicInputs,
  val l2L1Roots: List<ByteArray>,
  val filteredAddresses: List<ByteArray>,
) : BlockInterval {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupProofResponse

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!proof.contentEquals(other.proof)) return false
    if (publicInputs != other.publicInputs) return false
    if (!l2L1Roots.byteArrayListEquals(other.l2L1Roots)) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + proof.contentHashCode()
    result = 31 * result + publicInputs.hashCode()
    result = 31 * result + l2L1Roots.hashCode()
    result = 31 * result + filteredAddresses.hashCode()
    return result
  }
}
