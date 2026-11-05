package linea.clients

import linea.domain.BlockInterval
import linea.domain.BlockIntervalProofIndex
import linea.domain.StartBlockTimestampProvider
import linea.domain.assertConsecutiveIntervals
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import kotlin.time.Instant

data class RollupAggregationProofRequestV1(
  val rollupProofs: List<BlockIntervalProofIndex>,
) : BlockInterval, StartBlockTimestampProvider {
  init {
    assertConsecutiveIntervals(rollupProofs)
  }
  override val startBlockNumber: ULong
    get() = rollupProofs.first().startBlockNumber
  override val endBlockNumber: ULong
    get() = rollupProofs.last().endBlockNumber
  override val startBlockTimestamp: Instant
    get() = rollupProofs.first().startBlockTimestamp
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupAggregationProofRequestV1

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false
    if (rollupProofs != other.rollupProofs) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    result = 31 * result + rollupProofs.hashCode()
    return result
  }
}

/**
 * The PI tuple emitted by a rollup-aggregation proof (rollup_spec §2.4, `FinalizationPublicInput` in
 * `l1_rollup.py`) and submitted to L1 finalization.
 *
 * Shares its first 15 scalars with [RollupProofPublicInputs]. Where a rollup proof carries `l2L1Messages`,
 * `programVks` and `blockCount`, this carries the guest-built L2-to-L1 message trees ([l2L1Roots], each of depth
 * [l2L1TreeDepth]) and the guest [programIds] that L1 checks against its approved set.
 *
 * Domain twin of `lineth.coordinator.clients.prover.FinalizationPublicInputsDto`. Where the DTO uses `String` (hex)
 * this uses `ByteArray`, and where the DTO uses `Long` this uses `ULong`.
 */
data class FinalizationPublicInputs(
  val endBlockNumber: ULong,
  val endBlockTimestamp: Instant,
  val endL1L2BridgeMessageRollingHash: ByteArray,
  val endL1L2BridgeMessageNumber: ULong,
  val dynamicChainConfigHash: ByteArray,
  val parentFtxRollingHash: ByteArray,
  val parentFtxNumber: ULong,
  val endFtxRollingHash: ByteArray,
  val endFtxNumber: ULong,
  val parentDataRollingHash: ByteArray,
  val endDataRollingHash: ByteArray,
  val parentBlockHash: ByteArray,
  val endBlockHash: ByteArray,
  val parentDataTailTakeBytes: Int,
  val finalDataTailDiscardBytes: Int,
  val l2L1Roots: List<ByteArray>,
  val l2L1TreeDepth: ULong,
  val filteredAddresses: List<ByteArray>,
  /** Distinct, sorted ascending by byte value. */
  val programIds: List<ByteArray>,
  val l2MessagingBlocksOffsets: List<ULong>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as FinalizationPublicInputs

    if (endBlockNumber != other.endBlockNumber) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (!endL1L2BridgeMessageRollingHash.contentEquals(other.endL1L2BridgeMessageRollingHash)) return false
    if (endL1L2BridgeMessageNumber != other.endL1L2BridgeMessageNumber) return false
    if (!dynamicChainConfigHash.contentEquals(other.dynamicChainConfigHash)) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (parentFtxNumber != other.parentFtxNumber) return false
    if (!endFtxRollingHash.contentEquals(other.endFtxRollingHash)) return false
    if (endFtxNumber != other.endFtxNumber) return false
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (!endDataRollingHash.contentEquals(other.endDataRollingHash)) return false
    if (!parentBlockHash.contentEquals(other.parentBlockHash)) return false
    if (!endBlockHash.contentEquals(other.endBlockHash)) return false
    if (parentDataTailTakeBytes != other.parentDataTailTakeBytes) return false
    if (finalDataTailDiscardBytes != other.finalDataTailDiscardBytes) return false
    if (!l2L1Roots.byteArrayListEquals(other.l2L1Roots)) return false
    if (l2L1TreeDepth != other.l2L1TreeDepth) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false
    if (!programIds.byteArrayListEquals(other.programIds)) return false
    if (l2MessagingBlocksOffsets != other.l2MessagingBlocksOffsets) return false

    return true
  }

  override fun hashCode(): Int {
    var result = endBlockNumber.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + endL1L2BridgeMessageRollingHash.contentHashCode()
    result = 31 * result + endL1L2BridgeMessageNumber.hashCode()
    result = 31 * result + dynamicChainConfigHash.contentHashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + parentFtxNumber.hashCode()
    result = 31 * result + endFtxRollingHash.contentHashCode()
    result = 31 * result + endFtxNumber.hashCode()
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + endDataRollingHash.contentHashCode()
    result = 31 * result + parentBlockHash.contentHashCode()
    result = 31 * result + endBlockHash.contentHashCode()
    result = 31 * result + parentDataTailTakeBytes
    result = 31 * result + finalDataTailDiscardBytes
    result = 31 * result + l2L1Roots.byteArrayListHashCode()
    result = 31 * result + l2L1TreeDepth.hashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    result = 31 * result + programIds.byteArrayListHashCode()
    result = 31 * result + l2MessagingBlocksOffsets.hashCode()
    return result
  }
}

/**
 * Response of a rollup-aggregation proof (the final proof submitted to L1).
 *
 * Mirrors `lineth.coordinator.clients.prover.RollupAggregationProofResponseDto`: the DTO's `String` (hex) fields are
 * `ByteArray` here so a proof response — whether read from a JSON file or returned by a REST endpoint — deserializes
 * into the DTO and maps onto this domain type. L2-to-L1 roots, filtered addresses and messaging-block offsets all live
 * in [publicInputs]. [endBlockNumber] must equal `publicInputs.endBlockNumber`.
 */
data class RollupAggregationProofResponseV1(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val proof: ByteArray,
  val publicInputs: FinalizationPublicInputs,
) : BlockInterval {
  init {
    require(endBlockNumber == publicInputs.endBlockNumber) {
      "endBlockNumber=$endBlockNumber must equal publicInputs.endBlockNumber=${publicInputs.endBlockNumber}"
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupAggregationProofResponseV1

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!proof.contentEquals(other.proof)) return false
    if (publicInputs != other.publicInputs) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + proof.contentHashCode()
    result = 31 * result + publicInputs.hashCode()
    return result
  }
}
