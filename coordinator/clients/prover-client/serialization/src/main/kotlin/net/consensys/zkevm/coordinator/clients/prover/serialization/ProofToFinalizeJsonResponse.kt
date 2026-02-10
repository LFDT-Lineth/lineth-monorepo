package net.consensys.zkevm.coordinator.clients.prover.serialization

import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import com.fasterxml.jackson.databind.annotation.JsonSerialize
import net.consensys.zkevm.domain.ProofToFinalize
import kotlin.time.Instant

data class ProofToFinalizeJsonResponse(
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val aggregatedProof: ByteArray,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val parentStateRootHash: ByteArray,
  val aggregatedVerifierIndex: Int,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val aggregatedProofPublicInput: ByteArray,
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val dataHashes: List<ByteArray>,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val dataParentHash: ByteArray,
  val lastFinalizedBlockNumber: Long,
  val finalBlockNumber: Long,
  val parentAggregationLastBlockTimestamp: Long,
  val finalTimestamp: Long,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val l1RollingHash: ByteArray,
  val l1RollingHashMessageNumber: Long,
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val l2MerkleRoots: List<ByteArray>,
  val l2MerkleTreesDepth: Int,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val l2MessagingBlocksOffsets: ByteArray,
) {

  fun toDomainObject(): ProofToFinalize {
    return ProofToFinalize(
      aggregatedProof = aggregatedProof,
      parentStateRootHash = parentStateRootHash,
      aggregatedVerifierIndex = aggregatedVerifierIndex,
      aggregatedProofPublicInput = aggregatedProofPublicInput,
      dataHashes = dataHashes,
      dataParentHash = dataParentHash,
      firstBlockNumber = lastFinalizedBlockNumber.inc(),
      finalBlockNumber = finalBlockNumber,
      parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(parentAggregationLastBlockTimestamp),
      finalTimestamp = Instant.fromEpochSeconds(finalTimestamp),
      l1RollingHash = l1RollingHash,
      l1RollingHashMessageNumber = l1RollingHashMessageNumber,
      l2MerkleRoots = l2MerkleRoots,
      l2MerkleTreesDepth = l2MerkleTreesDepth,
      l2MessagingBlocksOffsets = l2MessagingBlocksOffsets,
    )
  }

  fun toJsonString(): String {
    return JsonSerialization.proofResponseMapperV1.writeValueAsString(this)
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ProofToFinalizeJsonResponse

    if (!aggregatedProof.contentEquals(other.aggregatedProof)) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (aggregatedVerifierIndex != other.aggregatedVerifierIndex) return false
    if (!aggregatedProofPublicInput.contentEquals(other.aggregatedProofPublicInput)) return false
    dataHashes.forEachIndexed { index, bytes ->
      if (!bytes.contentEquals(other.dataHashes[index])) return false
    }
    if (!dataParentHash.contentEquals(other.dataParentHash)) return false
    if (lastFinalizedBlockNumber != other.lastFinalizedBlockNumber) return false
    if (finalBlockNumber != other.finalBlockNumber) return false
    if (parentAggregationLastBlockTimestamp != other.parentAggregationLastBlockTimestamp) return false
    if (finalTimestamp != other.finalTimestamp) return false
    if (!l1RollingHash.contentEquals(other.l1RollingHash)) return false
    if (l1RollingHashMessageNumber != other.l1RollingHashMessageNumber) {
      return false
    }
    l2MerkleRoots.forEachIndexed { index, bytes ->
      if (!bytes.contentEquals(other.l2MerkleRoots[index])) return false
    }
    if (l2MerkleTreesDepth != other.l2MerkleTreesDepth) return false
    return l2MessagingBlocksOffsets.contentEquals(other.l2MessagingBlocksOffsets)
  }

  override fun hashCode(): Int {
    var result = aggregatedProof.contentHashCode()
    result = 31 * result + parentStateRootHash.contentHashCode()
    result = 31 * result + aggregatedVerifierIndex
    result = 31 * result + aggregatedProofPublicInput.contentHashCode()
    result = 31 * result + dataHashes.hashCode()
    result = 31 * result + dataParentHash.contentHashCode()
    result = 31 * result + lastFinalizedBlockNumber.hashCode()
    result = 31 * result + finalBlockNumber.hashCode()
    result = 31 * result + parentAggregationLastBlockTimestamp.hashCode()
    result = 31 * result + finalTimestamp.hashCode()
    result = 31 * result + l1RollingHash.contentHashCode()
    result = 31 * result + l1RollingHashMessageNumber.hashCode()
    result = 31 * result + l2MerkleRoots.hashCode()
    result = 31 * result + l2MerkleTreesDepth
    result = 31 * result + l2MessagingBlocksOffsets.contentHashCode()
    return result
  }

  companion object {

    val PROPERTIES_NOT_INCLUDED = setOf("aggregatedProverVersion")

    fun fromJsonString(jsonString: String): ProofToFinalizeJsonResponse {
      return JsonSerialization.proofResponseMapperV1.readValue(
        jsonString,
        ProofToFinalizeJsonResponse::class.java,
      )
    }

    fun fromDomainObject(proofToFinalize: ProofToFinalize): ProofToFinalizeJsonResponse {
      return ProofToFinalizeJsonResponse(
        aggregatedProof = proofToFinalize.aggregatedProof,
        parentStateRootHash = proofToFinalize.parentStateRootHash,
        aggregatedVerifierIndex = proofToFinalize.aggregatedVerifierIndex,
        aggregatedProofPublicInput = proofToFinalize.aggregatedProofPublicInput,
        dataHashes = proofToFinalize.dataHashes,
        dataParentHash = proofToFinalize.dataParentHash,
        finalBlockNumber = proofToFinalize.finalBlockNumber,
        parentAggregationLastBlockTimestamp = proofToFinalize.parentAggregationLastBlockTimestamp.epochSeconds,
        lastFinalizedBlockNumber = proofToFinalize.firstBlockNumber.dec(),
        finalTimestamp = proofToFinalize.finalTimestamp.epochSeconds,
        l1RollingHash = proofToFinalize.l1RollingHash,
        l1RollingHashMessageNumber = proofToFinalize.l1RollingHashMessageNumber,
        l2MerkleRoots = proofToFinalize.l2MerkleRoots,
        l2MerkleTreesDepth = proofToFinalize.l2MerkleTreesDepth,
        l2MessagingBlocksOffsets = proofToFinalize.l2MessagingBlocksOffsets,
      )
    }
  }
}
