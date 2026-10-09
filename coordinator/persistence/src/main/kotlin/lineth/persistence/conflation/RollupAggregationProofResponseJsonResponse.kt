package lineth.persistence.conflation

import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import com.fasterxml.jackson.databind.annotation.JsonSerialize
import linea.clients.RollupAggregationProofResponseV1
import linea.clients.RollupProofPublicInputs
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import lineth.coordinator.clients.prover.serialization.ByteArrayDeserializer
import lineth.coordinator.clients.prover.serialization.ByteArraySerializer
import lineth.coordinator.clients.prover.serialization.JsonSerialization
import kotlin.time.Instant

data class RollupAggregationProofResponseJsonResponse(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val proof: ByteArray,
  // RollupProofPublicInputs fields (flat)
  val endBlockTimestamp: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val l2L1BridgeTransactionTree: ByteArray,
  val parentL1L2BridgeMessageNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val parentL1L2BridgeMessageRollingHash: ByteArray,
  val endL1L2BridgeMessageNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val endL1L2BridgeMessageRollingHash: ByteArray,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val dynamicChainConfigHash: ByteArray,
  val parentFtxNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val parentFtxRollingHash: ByteArray,
  val endFtxNumber: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val endFtxRollingHash: ByteArray,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val filteredAddressesHash: ByteArray,
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
  val startOffset: Int,
  val endOffset: Int,
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val programVks: List<ByteArray>,
  val l2L1TreeDepth: Int?,
  // RollupAggregationProofResponseV1 fields
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val l2L1Roots: List<ByteArray>,
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val filteredAddresses: List<ByteArray>,
  val l2MessagingBlocksOffsets: List<Long>,
) {

  fun toDomainObject(): RollupAggregationProofResponseV1 {
    return RollupAggregationProofResponseV1(
      startBlockNumber = startBlockNumber.toULong(),
      endBlockNumber = endBlockNumber.toULong(),
      proof = proof,
      publicInputs = RollupProofPublicInputs(
        endBlockNumber = endBlockNumber.toULong(),
        endBlockTimestamp = Instant.fromEpochSeconds(endBlockTimestamp),
        l2L1BridgeTransactionTree = l2L1BridgeTransactionTree,
        parentL1L2BridgeMessageNumber = parentL1L2BridgeMessageNumber.toULong(),
        parentL1L2BridgeMessageRollingHash = parentL1L2BridgeMessageRollingHash,
        endL1L2BridgeMessageNumber = endL1L2BridgeMessageNumber.toULong(),
        endL1L2BridgeMessageRollingHash = endL1L2BridgeMessageRollingHash,
        dynamicChainConfigHash = dynamicChainConfigHash,
        parentFtxNumber = parentFtxNumber.toULong(),
        parentFtxRollingHash = parentFtxRollingHash,
        endFtxNumber = endFtxNumber.toULong(),
        endFtxRollingHash = endFtxRollingHash,
        filteredAddressesHash = filteredAddressesHash,
        parentDataRollingHash = parentDataRollingHash,
        endDataRollingHash = endDataRollingHash,
        parentBlockHash = parentBlockHash,
        endBlockHash = endBlockHash,
        startOffset = startOffset,
        endOffset = endOffset,
        programVks = programVks,
        l2L1TreeDepth = l2L1TreeDepth,
      ),
      l2L1Roots = l2L1Roots,
      filteredAddresses = filteredAddresses,
      l2MessagingBlocksOffsets = l2MessagingBlocksOffsets.map { it.toULong() },
    )
  }

  fun toJsonString(): String {
    return JsonSerialization.proofResponseMapperV1.writeValueAsString(this)
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupAggregationProofResponseJsonResponse

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (parentL1L2BridgeMessageNumber != other.parentL1L2BridgeMessageNumber) return false
    if (endL1L2BridgeMessageNumber != other.endL1L2BridgeMessageNumber) return false
    if (parentFtxNumber != other.parentFtxNumber) return false
    if (endFtxNumber != other.endFtxNumber) return false
    if (startOffset != other.startOffset) return false
    if (endOffset != other.endOffset) return false
    if (l2L1TreeDepth != other.l2L1TreeDepth) return false
    if (l2MessagingBlocksOffsets != other.l2MessagingBlocksOffsets) return false
    if (!proof.contentEquals(other.proof)) return false
    if (!l2L1BridgeTransactionTree.contentEquals(other.l2L1BridgeTransactionTree)) return false
    if (!parentL1L2BridgeMessageRollingHash.contentEquals(other.parentL1L2BridgeMessageRollingHash)) return false
    if (!endL1L2BridgeMessageRollingHash.contentEquals(other.endL1L2BridgeMessageRollingHash)) return false
    if (!dynamicChainConfigHash.contentEquals(other.dynamicChainConfigHash)) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (!endFtxRollingHash.contentEquals(other.endFtxRollingHash)) return false
    if (!filteredAddressesHash.contentEquals(other.filteredAddressesHash)) return false
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (!endDataRollingHash.contentEquals(other.endDataRollingHash)) return false
    if (!parentBlockHash.contentEquals(other.parentBlockHash)) return false
    if (!endBlockHash.contentEquals(other.endBlockHash)) return false
    if (!programVks.byteArrayListEquals(other.programVks)) return false
    if (!l2L1Roots.byteArrayListEquals(other.l2L1Roots)) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + parentL1L2BridgeMessageNumber.hashCode()
    result = 31 * result + endL1L2BridgeMessageNumber.hashCode()
    result = 31 * result + parentFtxNumber.hashCode()
    result = 31 * result + endFtxNumber.hashCode()
    result = 31 * result + startOffset.hashCode()
    result = 31 * result + endOffset.hashCode()
    result = 31 * result + (l2L1TreeDepth ?: 0)
    result = 31 * result + l2MessagingBlocksOffsets.hashCode()
    result = 31 * result + proof.contentHashCode()
    result = 31 * result + l2L1BridgeTransactionTree.contentHashCode()
    result = 31 * result + parentL1L2BridgeMessageRollingHash.contentHashCode()
    result = 31 * result + endL1L2BridgeMessageRollingHash.contentHashCode()
    result = 31 * result + dynamicChainConfigHash.contentHashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + endFtxRollingHash.contentHashCode()
    result = 31 * result + filteredAddressesHash.contentHashCode()
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + endDataRollingHash.contentHashCode()
    result = 31 * result + parentBlockHash.contentHashCode()
    result = 31 * result + endBlockHash.contentHashCode()
    result = 31 * result + programVks.byteArrayListHashCode()
    result = 31 * result + l2L1Roots.byteArrayListHashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    return result
  }

  companion object {
    fun fromJsonString(jsonString: String): RollupAggregationProofResponseJsonResponse {
      return JsonSerialization.proofResponseMapperV1.readValue(
        jsonString,
        RollupAggregationProofResponseJsonResponse::class.java,
      )
    }

    fun fromDomainObject(proof: RollupAggregationProofResponseV1): RollupAggregationProofResponseJsonResponse {
      val pi = proof.publicInputs
      return RollupAggregationProofResponseJsonResponse(
        startBlockNumber = proof.startBlockNumber.toLong(),
        endBlockNumber = proof.endBlockNumber.toLong(),
        proof = proof.proof,
        endBlockTimestamp = pi.endBlockTimestamp.epochSeconds,
        l2L1BridgeTransactionTree = pi.l2L1BridgeTransactionTree,
        parentL1L2BridgeMessageNumber = pi.parentL1L2BridgeMessageNumber.toLong(),
        parentL1L2BridgeMessageRollingHash = pi.parentL1L2BridgeMessageRollingHash,
        endL1L2BridgeMessageNumber = pi.endL1L2BridgeMessageNumber.toLong(),
        endL1L2BridgeMessageRollingHash = pi.endL1L2BridgeMessageRollingHash,
        dynamicChainConfigHash = pi.dynamicChainConfigHash,
        parentFtxNumber = pi.parentFtxNumber.toLong(),
        parentFtxRollingHash = pi.parentFtxRollingHash,
        endFtxNumber = pi.endFtxNumber.toLong(),
        endFtxRollingHash = pi.endFtxRollingHash,
        filteredAddressesHash = pi.filteredAddressesHash,
        parentDataRollingHash = pi.parentDataRollingHash,
        endDataRollingHash = pi.endDataRollingHash,
        parentBlockHash = pi.parentBlockHash,
        endBlockHash = pi.endBlockHash,
        startOffset = pi.startOffset,
        endOffset = pi.endOffset,
        programVks = pi.programVks,
        l2L1TreeDepth = pi.l2L1TreeDepth,
        l2L1Roots = proof.l2L1Roots,
        filteredAddresses = proof.filteredAddresses,
        l2MessagingBlocksOffsets = proof.l2MessagingBlocksOffsets.map { it.toLong() },
      )
    }
  }
}
