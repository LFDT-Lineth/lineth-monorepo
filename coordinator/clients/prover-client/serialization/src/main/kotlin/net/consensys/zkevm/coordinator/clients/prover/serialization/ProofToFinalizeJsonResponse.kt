package net.consensys.zkevm.coordinator.clients.prover.serialization

import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import com.fasterxml.jackson.databind.annotation.JsonSerialize
import linea.domain.ProofToFinalize
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
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
  val parentAggregationFtxNumber: Long = 0,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val parentAggregationFtxRollingHash: ByteArray = ByteArray(32),
  val finalFtxNumber: Long = 0,
  @JsonSerialize(using = ByteArraySerializer::class)
  @JsonDeserialize(using = ByteArrayDeserializer::class)
  val finalFtxRollingHash: ByteArray = ByteArray(32),
  @JsonSerialize(contentUsing = ByteArraySerializer::class)
  @JsonDeserialize(contentUsing = ByteArrayDeserializer::class)
  val filteredAddresses: List<ByteArray> = emptyList(),
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
      parentAggregationFtxNumber = parentAggregationFtxNumber.toULong(),
      parentAggregationFtxRollingHash = parentAggregationFtxRollingHash,
      finalFtxNumber = finalFtxNumber.toULong(),
      finalFtxRollingHash = finalFtxRollingHash,
      filteredAddresses = filteredAddresses,
    )
  }

  fun toJsonString(): String {
    return JsonSerialization.proofResponseMapperV1.writeValueAsString(this)
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ProofToFinalizeJsonResponse

    if (aggregatedVerifierIndex != other.aggregatedVerifierIndex) return false
    if (lastFinalizedBlockNumber != other.lastFinalizedBlockNumber) return false
    if (finalBlockNumber != other.finalBlockNumber) return false
    if (parentAggregationLastBlockTimestamp != other.parentAggregationLastBlockTimestamp) return false
    if (finalTimestamp != other.finalTimestamp) return false
    if (l1RollingHashMessageNumber != other.l1RollingHashMessageNumber) return false
    if (l2MerkleTreesDepth != other.l2MerkleTreesDepth) return false
    if (parentAggregationFtxNumber != other.parentAggregationFtxNumber) return false
    if (finalFtxNumber != other.finalFtxNumber) return false
    if (!aggregatedProof.contentEquals(other.aggregatedProof)) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (!aggregatedProofPublicInput.contentEquals(other.aggregatedProofPublicInput)) return false
    if (!dataHashes.byteArrayListEquals(other.dataHashes)) return false
    if (!dataParentHash.contentEquals(other.dataParentHash)) return false
    if (!l1RollingHash.contentEquals(other.l1RollingHash)) return false
    if (!l2MerkleRoots.byteArrayListEquals(other.l2MerkleRoots)) return false
    if (!l2MessagingBlocksOffsets.contentEquals(other.l2MessagingBlocksOffsets)) return false
    if (!parentAggregationFtxRollingHash.contentEquals(other.parentAggregationFtxRollingHash)) return false
    if (!finalFtxRollingHash.contentEquals(other.finalFtxRollingHash)) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = aggregatedVerifierIndex
    result = 31 * result + lastFinalizedBlockNumber.hashCode()
    result = 31 * result + finalBlockNumber.hashCode()
    result = 31 * result + parentAggregationLastBlockTimestamp.hashCode()
    result = 31 * result + finalTimestamp.hashCode()
    result = 31 * result + l1RollingHashMessageNumber.hashCode()
    result = 31 * result + l2MerkleTreesDepth
    result = 31 * result + parentAggregationFtxNumber.hashCode()
    result = 31 * result + finalFtxNumber.hashCode()
    result = 31 * result + aggregatedProof.contentHashCode()
    result = 31 * result + parentStateRootHash.contentHashCode()
    result = 31 * result + aggregatedProofPublicInput.contentHashCode()
    result = 31 * result + dataHashes.byteArrayListHashCode()
    result = 31 * result + dataParentHash.contentHashCode()
    result = 31 * result + l1RollingHash.contentHashCode()
    result = 31 * result + l2MerkleRoots.byteArrayListHashCode()
    result = 31 * result + l2MessagingBlocksOffsets.contentHashCode()
    result = 31 * result + parentAggregationFtxRollingHash.contentHashCode()
    result = 31 * result + finalFtxRollingHash.contentHashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    return result
  }

  companion object {

    val optionalFields = setOf(
      "parentAggregationFtxNumber",
      "parentAggregationFtxRollingHash",
      "finalFtxNumber",
      "finalFtxRollingHash",
      "filteredAddresses",
    )

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
        parentAggregationFtxNumber = proofToFinalize.parentAggregationFtxNumber.toLong(),
        parentAggregationFtxRollingHash = proofToFinalize.parentAggregationFtxRollingHash,
        finalFtxNumber = proofToFinalize.finalFtxNumber.toLong(),
        finalFtxRollingHash = proofToFinalize.finalFtxRollingHash,
        filteredAddresses = proofToFinalize.filteredAddresses,
      )
    }
  }
}
