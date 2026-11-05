package lineth.coordinator.clients.prover.serialization

import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import com.fasterxml.jackson.databind.annotation.JsonSerialize
import linea.clients.FinalizationPublicInputs
import linea.clients.RollupAggregationProofResponseV1
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import kotlin.time.Instant

/**
 * Persisted JSON form of a [RollupAggregationProofResponseV1]: the RISC-V rollup-aggregation proof and its
 * finalization public inputs, as stored in the `aggregations.aggregation_proof` column. The RISC-V counterpart of
 * [ProofToFinalizeJsonResponse].
 *
 * Flat: [startBlockNumber] and [proof], then the [FinalizationPublicInputs] fields under their domain names.
 * [endBlockNumber] is stored once and restores both `RollupAggregationProofResponseV1.endBlockNumber` and
 * `publicInputs.endBlockNumber`, which [RollupAggregationProofResponseV1] requires to be equal.
 */
data class RollupAggregationProofResponseV1JsonResponse(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val proof: ByteArray,
  val endBlockTimestamp: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val endL1L2BridgeMessageRollingHash: ByteArray,
  val endL1L2BridgeMessageNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val dynamicChainConfigHash: ByteArray,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val parentFtxRollingHash: ByteArray,
  val parentFtxNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val endFtxRollingHash: ByteArray,
  val endFtxNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val parentDataRollingHash: ByteArray,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val endDataRollingHash: ByteArray,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val parentBlockHash: ByteArray,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val endBlockHash: ByteArray,
  val parentDataTailTakeBytes: Int,
  val finalDataTailDiscardBytes: Int,
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val l2L1Roots: List<ByteArray>,
  val l2L1TreeDepth: Long,
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val filteredAddresses: List<ByteArray>,
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val programIds: List<ByteArray>,
  val l2MessagingBlocksOffsets: List<Long>,
) {

  fun toDomainObject(): RollupAggregationProofResponseV1 {
    return RollupAggregationProofResponseV1(
      startBlockNumber = startBlockNumber.toULong(),
      endBlockNumber = endBlockNumber.toULong(),
      proof = proof,
      publicInputs = FinalizationPublicInputs(
        endBlockNumber = endBlockNumber.toULong(),
        endBlockTimestamp = Instant.fromEpochSeconds(endBlockTimestamp),
        endL1L2BridgeMessageRollingHash = endL1L2BridgeMessageRollingHash,
        endL1L2BridgeMessageNumber = endL1L2BridgeMessageNumber.toULong(),
        dynamicChainConfigHash = dynamicChainConfigHash,
        parentFtxRollingHash = parentFtxRollingHash,
        parentFtxNumber = parentFtxNumber.toULong(),
        endFtxRollingHash = endFtxRollingHash,
        endFtxNumber = endFtxNumber.toULong(),
        parentDataRollingHash = parentDataRollingHash,
        endDataRollingHash = endDataRollingHash,
        parentBlockHash = parentBlockHash,
        endBlockHash = endBlockHash,
        parentDataTailTakeBytes = parentDataTailTakeBytes,
        finalDataTailDiscardBytes = finalDataTailDiscardBytes,
        l2L1Roots = l2L1Roots,
        l2L1TreeDepth = l2L1TreeDepth.toULong(),
        filteredAddresses = filteredAddresses,
        programIds = programIds,
        l2MessagingBlocksOffsets = l2MessagingBlocksOffsets.map { it.toULong() },
      ),
    )
  }

  fun toJsonString(): String {
    return JsonSerialization.proofResponseMapperV1.writeValueAsString(this)
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupAggregationProofResponseV1JsonResponse

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!proof.contentEquals(other.proof)) return false
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
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + proof.contentHashCode()
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

  companion object {
    fun fromJsonString(jsonString: String): RollupAggregationProofResponseV1JsonResponse {
      return JsonSerialization.proofResponseMapperV1.readValue(
        jsonString,
        RollupAggregationProofResponseV1JsonResponse::class.java,
      )
    }

    fun fromDomainObject(response: RollupAggregationProofResponseV1): RollupAggregationProofResponseV1JsonResponse {
      val pi = response.publicInputs
      return RollupAggregationProofResponseV1JsonResponse(
        startBlockNumber = response.startBlockNumber.toLong(),
        endBlockNumber = response.endBlockNumber.toLong(),
        proof = response.proof,
        endBlockTimestamp = pi.endBlockTimestamp.epochSeconds,
        endL1L2BridgeMessageRollingHash = pi.endL1L2BridgeMessageRollingHash,
        endL1L2BridgeMessageNumber = pi.endL1L2BridgeMessageNumber.toLong(),
        dynamicChainConfigHash = pi.dynamicChainConfigHash,
        parentFtxRollingHash = pi.parentFtxRollingHash,
        parentFtxNumber = pi.parentFtxNumber.toLong(),
        endFtxRollingHash = pi.endFtxRollingHash,
        endFtxNumber = pi.endFtxNumber.toLong(),
        parentDataRollingHash = pi.parentDataRollingHash,
        endDataRollingHash = pi.endDataRollingHash,
        parentBlockHash = pi.parentBlockHash,
        endBlockHash = pi.endBlockHash,
        parentDataTailTakeBytes = pi.parentDataTailTakeBytes,
        finalDataTailDiscardBytes = pi.finalDataTailDiscardBytes,
        l2L1Roots = pi.l2L1Roots,
        l2L1TreeDepth = pi.l2L1TreeDepth.toLong(),
        filteredAddresses = pi.filteredAddresses,
        programIds = pi.programIds,
        l2MessagingBlocksOffsets = pi.l2MessagingBlocksOffsets.map { it.toLong() },
      )
    }
  }
}
