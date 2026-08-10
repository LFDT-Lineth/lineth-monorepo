package linea.clients

import linea.domain.BlockInterval
import linea.domain.BlockIntervalProofIndex
import linea.domain.StartBlockTimestampProvider
import linea.domain.assertConsecutiveIntervals
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import kotlin.time.Instant

data class RollupProofRequestV1(
  val blobs: List<BlobWitness>,
  val parentShnarf: ByteArray,
  val endShnarf: ByteArray,
  val l2Executions: List<BlockIntervalProofIndex>,
) : BlockInterval, StartBlockTimestampProvider {
  init {
    assertConsecutiveIntervals(blobs)
    assertConsecutiveIntervals(l2Executions)
  }

  override val startBlockNumber: ULong
    get() = l2Executions.first().startBlockNumber
  override val endBlockNumber: ULong
    get() = l2Executions.last().endBlockNumber
  override val startBlockTimestamp: Instant
    get() = l2Executions.first().startBlockTimestamp

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
    if (l2Executions != other.l2Executions) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    result = 31 * result + blobs.hashCode()
    result = 31 * result + parentShnarf.contentHashCode()
    result = 31 * result + endShnarf.contentHashCode()
    result = 31 * result + l2Executions.hashCode()
    return result
  }
}

data class BlobWitness(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val blobHash: ByteArray,
  val blobKzgProof: ByteArray,
  val blockRlps: List<ByteArray>,
) : BlockInterval {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobWitness

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!blobHash.contentEquals(other.blobHash)) return false
    if (!blobKzgProof.contentEquals(other.blobKzgProof)) return false
    if (!blockRlps.byteArrayListEquals(other.blockRlps)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + blobHash.contentHashCode()
    result = 31 * result + blobKzgProof.contentHashCode()
    result = 31 * result + blockRlps.byteArrayListHashCode()
    return result
  }
}

/**
 * The 14-field PI tuple emitted by a rollup / rollup-aggregation proof (rollup_spec §2.4).
 *
 * Domain twin of `lineth.coordinator.clients.prover.riscv.RollupPublicInputsDto`. Kept here (rather than reusing the
 * DTO) because this module is depended upon by the prover-client modules, not the other way around. Where the DTO
 * uses `String` (hex) this uses `ByteArray`, and where the DTO uses `Long` this uses `ULong`.
 */
data class RollupProofPublicInputs(
  val endBlockNumber: ULong,
  val endBlockTimestamp: Instant,
  val l2L1BridgeTransactionTree: ByteArray,
  val parentL1L2BridgeMessageNumber: ULong,
  val parentL1L2BridgeMessageRollingHash: ByteArray,
  val endL1L2BridgeMessageNumber: ULong,
  val endL1L2BridgeMessageRollingHash: ByteArray,
  val dynamicChainConfigHash: ByteArray,
  val parentFtxNumber: ULong,
  val parentFtxRollingHash: ByteArray,
  val endFtxNumber: ULong,
  val endFtxRollingHash: ByteArray,
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
    if (!parentL1L2BridgeMessageRollingHash.contentEquals(other.parentL1L2BridgeMessageRollingHash)) return false
    if (parentL1L2BridgeMessageNumber != other.parentL1L2BridgeMessageNumber) return false
    if (!endL1L2BridgeMessageRollingHash.contentEquals(other.endL1L2BridgeMessageRollingHash)) return false
    if (endL1L2BridgeMessageNumber != other.endL1L2BridgeMessageNumber) return false
    if (!dynamicChainConfigHash.contentEquals(other.dynamicChainConfigHash)) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (parentFtxNumber != other.parentFtxNumber) return false
    if (!endFtxRollingHash.contentEquals(other.endFtxRollingHash)) return false
    if (endFtxNumber != other.endFtxNumber) return false
    if (!filteredAddressesHash.contentEquals(other.filteredAddressesHash)) return false
    if (!parentShnarf.contentEquals(other.parentShnarf)) return false
    if (!endShnarf.contentEquals(other.endShnarf)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = endBlockNumber.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + l2L1BridgeTransactionTree.contentHashCode()
    result = 31 * result + parentL1L2BridgeMessageRollingHash.contentHashCode()
    result = 31 * result + parentL1L2BridgeMessageNumber.hashCode()
    result = 31 * result + endL1L2BridgeMessageRollingHash.contentHashCode()
    result = 31 * result + endL1L2BridgeMessageNumber.hashCode()
    result = 31 * result + dynamicChainConfigHash.contentHashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + parentFtxNumber.hashCode()
    result = 31 * result + endFtxRollingHash.contentHashCode()
    result = 31 * result + endFtxNumber.hashCode()
    result = 31 * result + filteredAddressesHash.contentHashCode()
    result = 31 * result + parentShnarf.contentHashCode()
    result = 31 * result + endShnarf.contentHashCode()
    return result
  }
}

/**
 * Response of a rollup proof.
 *
 * Mirrors `lineth.coordinator.clients.prover.riscv.RollupProofResponseDto`: the DTO's `String` (hex) fields are
 * `ByteArray` here so a proof response — whether read from a JSON file or returned by a REST endpoint — deserializes
 * into the DTO and maps onto this domain type.
 */
data class RollupProofResponseV1(
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

    other as RollupProofResponseV1

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
    result = 31 * result + l2L1Roots.byteArrayListHashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    return result
  }
}
