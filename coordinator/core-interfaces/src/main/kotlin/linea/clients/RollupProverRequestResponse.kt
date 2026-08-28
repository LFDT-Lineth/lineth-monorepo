package linea.clients

import linea.domain.BlockInterval
import linea.domain.BlockIntervalProofIndex
import linea.domain.StartBlockTimestampProvider
import linea.domain.assertConsecutiveIntervals
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import kotlin.time.Instant

data class RollupProofRequestV1(
  val conflations: List<ConflationWitness>,
  val l2Executions: List<BlockIntervalProofIndex>,
  val chunks: List<ByteArray>,
  val parentDataRollingHash: ByteArray,
  val startOffset: Int,
  val opaquePrefixBytes: ByteArray = ByteArray(0),
  val opaqueSuffixBytes: ByteArray = ByteArray(0),
  val boundaryPrevDataRollingHash: ByteArray? = null,
) : BlockInterval, StartBlockTimestampProvider {
  init {
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
    if (conflations != other.conflations) return false
    if (l2Executions != other.l2Executions) return false
    if (!chunks.byteArrayListEquals(other.chunks)) return false
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (startOffset != other.startOffset) return false
    if (!opaquePrefixBytes.contentEquals(other.opaquePrefixBytes)) return false
    if (!opaqueSuffixBytes.contentEquals(other.opaqueSuffixBytes)) return false
    if (!boundaryPrevDataRollingHash.contentEquals(other.boundaryPrevDataRollingHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    result = 31 * result + conflations.hashCode()
    result = 31 * result + l2Executions.hashCode()
    result = 31 * result + chunks.byteArrayListHashCode()
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + startOffset.hashCode()
    result = 31 * result + opaquePrefixBytes.contentHashCode()
    result = 31 * result + opaqueSuffixBytes.contentHashCode()
    result = 31 * result + (boundaryPrevDataRollingHash?.contentHashCode() ?: 0)
    return result
  }
}

data class ConflationWitness(
  val blockRlps: List<ByteArray>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ConflationWitness

    if (!blockRlps.byteArrayListEquals(other.blockRlps)) return false

    return true
  }

  override fun hashCode(): Int = blockRlps.byteArrayListHashCode()
}

/**
 * The 20-field PI tuple emitted by a rollup / rollup-aggregation proof (rollup_spec §2.4).
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
  val parentDataRollingHash: ByteArray,
  val endDataRollingHash: ByteArray,
  val parentBlockHash: ByteArray,
  val endBlockHash: ByteArray,
  val startOffset: Int,
  val endOffset: Int,
  val programVks: List<ByteArray>,
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
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (!endDataRollingHash.contentEquals(other.endDataRollingHash)) return false
    if (!parentBlockHash.contentEquals(other.parentBlockHash)) return false
    if (!endBlockHash.contentEquals(other.endBlockHash)) return false
    if (startOffset != other.startOffset) return false
    if (endOffset != other.endOffset) return false
    if (!programVks.byteArrayListEquals(other.programVks)) return false

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
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + endDataRollingHash.contentHashCode()
    result = 31 * result + parentBlockHash.contentHashCode()
    result = 31 * result + endBlockHash.contentHashCode()
    result = 31 * result + startOffset.hashCode()
    result = 31 * result + endOffset.hashCode()
    result = 31 * result + programVks.byteArrayListHashCode()
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
  val programVk: ByteArray,
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
    if (!programVk.contentEquals(other.programVk)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + proof.contentHashCode()
    result = 31 * result + publicInputs.hashCode()
    result = 31 * result + l2L1Roots.byteArrayListHashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    result = 31 * result + programVk.contentHashCode()
    return result
  }
}
