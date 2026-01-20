package net.consensys.zkevm.domain

import kotlinx.datetime.Instant
import linea.domain.BlockInterval
import linea.domain.BlockIntervals

typealias BlobsToAggregate = BlockInterval

/**
 * Represents an Aggregation request to the Prover
 * @property parentAggregationLastBlockTimestamp The timestamp of the last block of the previous aggregation.
 */
data class ProofsToAggregate(
  val compressionProofIndexes: List<ProofIndex>,
  val executionProofs: BlockIntervals,
  val invalidityProofs: List<ProofIndex> = emptyList(),
  val parentAggregationLastBlockTimestamp: Instant,
  val parentAggregationLastL1RollingHashMessageNumber: ULong,
  val parentAggregationLastL1RollingHash: ByteArray,
) : BlockInterval {
  override val startBlockNumber = compressionProofIndexes.first().startBlockNumber
  override val endBlockNumber = compressionProofIndexes.last().endBlockNumber
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ProofsToAggregate

    if (compressionProofIndexes != other.compressionProofIndexes) return false
    if (executionProofs != other.executionProofs) return false
    if (invalidityProofs != other.invalidityProofs) return false
    if (parentAggregationLastBlockTimestamp != other.parentAggregationLastBlockTimestamp) return false
    if (parentAggregationLastL1RollingHashMessageNumber != other.parentAggregationLastL1RollingHashMessageNumber) {
      return false
    }
    if (!parentAggregationLastL1RollingHash.contentEquals(other.parentAggregationLastL1RollingHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = compressionProofIndexes.hashCode()
    result = 31 * result + executionProofs.hashCode()
    result = 31 * result + invalidityProofs.hashCode()
    result = 31 * result + parentAggregationLastBlockTimestamp.hashCode()
    result = 31 * result + parentAggregationLastL1RollingHashMessageNumber.hashCode()
    result = 31 * result + parentAggregationLastL1RollingHash.contentHashCode()
    return result
  }
}

data class ProofToFinalize(
  val aggregatedProof: ByteArray,
  val parentStateRootHash: ByteArray,
  val aggregatedVerifierIndex: Int,
  val aggregatedProofPublicInput: ByteArray,
  val dataHashes: List<ByteArray>,
  val dataParentHash: ByteArray,
  val firstBlockNumber: Long,
  val finalBlockNumber: Long,
  val parentAggregationLastBlockTimestamp: Instant,
  val finalTimestamp: Instant,
  val l1RollingHash: ByteArray,
  val l1RollingHashMessageNumber: Long,
  val l2MerkleRoots: List<ByteArray>,
  val l2MerkleTreesDepth: Int,
  val l2MessagingBlocksOffsets: ByteArray,
) : BlockInterval {
  override val startBlockNumber: ULong = firstBlockNumber.toULong()
  override val endBlockNumber: ULong = finalBlockNumber.toULong()

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ProofToFinalize

    if (!aggregatedProof.contentEquals(other.aggregatedProof)) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (aggregatedVerifierIndex != other.aggregatedVerifierIndex) return false
    if (!aggregatedProofPublicInput.contentEquals(other.aggregatedProofPublicInput)) return false
    dataHashes.forEachIndexed { index, bytes ->
      if (!bytes.contentEquals(other.dataHashes[index])) return false
    }
    if (!dataParentHash.contentEquals(other.dataParentHash)) return false
    if (firstBlockNumber != other.firstBlockNumber) return false
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
    result = 31 * result + firstBlockNumber.hashCode()
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
}

data class Aggregation(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val batchCount: ULong,
  val aggregationProof: ProofToFinalize?,
) : BlockInterval {
  enum class Status {
    Proven,
    Proving,
  }
}

data class FinalizationSubmittedEvent(
  val aggregationProof: ProofToFinalize,
  val parentShnarf: ByteArray,
  val parentL1RollingHash: ByteArray,
  val parentL1RollingHashMessageNumber: Long,
  val submissionTimestamp: Instant,
  val transactionHash: ByteArray,
) : BlockInterval by aggregationProof {

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as FinalizationSubmittedEvent

    if (aggregationProof != other.aggregationProof) return false
    if (!parentShnarf.contentEquals(other.parentShnarf)) return false
    if (!parentL1RollingHash.contentEquals(other.parentL1RollingHash)) return false
    if (parentL1RollingHashMessageNumber != other.parentL1RollingHashMessageNumber) return false
    if (submissionTimestamp != other.submissionTimestamp) return false
    if (transactionHash.contentEquals(other.transactionHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = aggregationProof.hashCode()
    result = 31 * result + parentShnarf.contentHashCode()
    result = 31 * result + parentL1RollingHash.contentHashCode()
    result = 31 * result + parentL1RollingHashMessageNumber.hashCode()
    result = 31 * result + submissionTimestamp.hashCode()
    result = 31 * result + transactionHash.contentHashCode()
    return result
  }

  fun getSubmissionDelay(): Long {
    return submissionTimestamp.minus(aggregationProof.finalTimestamp).inWholeSeconds
  }
}
