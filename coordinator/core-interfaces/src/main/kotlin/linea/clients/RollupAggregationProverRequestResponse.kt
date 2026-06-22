package linea.clients

import linea.domain.BlockInterval
import linea.domain.StartBlockTimestampProvider
import linea.kotlin.byteArrayListEquals
import kotlin.time.Instant

typealias RollupAggregationPublicInputs = RollupProofPublicInputs

data class RollupAggregationProofRequestV1(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  override val startBlockTimestamp: Instant,
  val rollupProofs: List<RollupProofResponse>,
) : BlockInterval, StartBlockTimestampProvider {
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
 * Response of a rollup-aggregation proof.
 *
 * Mirrors `linea.coordinator.clients.prover.riscv.RollupAggregationProofResponseDto`: the DTO's `String` (hex) proof
 * is `ByteArray` here so a proof response — whether read from a JSON file or returned by a REST endpoint —
 * deserializes into the DTO and maps onto this domain type.
 */
data class RollupAggregationProofResponse(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val proof: ByteArray,
  val publicInputs: RollupAggregationPublicInputs,
  val l2L1Roots: List<ByteArray>,
  val filteredAddresses: List<ByteArray>,
  val l2MessagingBlocksOffsets: List<ULong>,
) : BlockInterval {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupAggregationProofResponse

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!proof.contentEquals(other.proof)) return false
    if (publicInputs != other.publicInputs) return false
    if (!l2L1Roots.byteArrayListEquals(other.l2L1Roots)) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false
    if (l2MessagingBlocksOffsets != other.l2MessagingBlocksOffsets) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + proof.contentHashCode()
    result = 31 * result + publicInputs.hashCode()
    result = 31 * result + l2L1Roots.hashCode()
    result = 31 * result + filteredAddresses.hashCode()
    result = 31 * result + l2MessagingBlocksOffsets.hashCode()
    return result
  }
}
