package linea.domain

import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import linea.kotlin.encodeHex
import kotlin.time.Instant

typealias BlobsToAggregate = BlockInterval

/**
 * Represents an Aggregation request to the Prover
 * @property parentAggregationLastBlockTimestamp The timestamp of the last block of the previous aggregation.
 * @property startBlockTimestamp L2 time of the first block in this aggregation span (coordinator routing).
 */
data class ProofsToAggregate(
  val compressionProofIndexes: List<CompressionProofIndex>,
  val executionProofs: BlockIntervals,
  val invalidityProofs: List<InvalidityProofIndex>,
  val parentAggregationLastBlockTimestamp: Instant,
  val parentAggregationLastL1RollingHashMessageNumber: ULong,
  val parentAggregationLastL1RollingHash: ByteArray,
  val parentAggregationLastFtxNumber: ULong,
  val parentAggregationLastFtxRollingHash: ByteArray,
  override val startBlockTimestamp: Instant,
) : BlockInterval, StartBlockTimestampProvider {
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
    if (parentAggregationLastFtxNumber != other.parentAggregationLastFtxNumber) return false
    if (!parentAggregationLastFtxRollingHash.contentEquals(other.parentAggregationLastFtxRollingHash)) return false
    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false

    return true
  }

  override fun hashCode(): Int {
    var result = compressionProofIndexes.hashCode()
    result = 31 * result + executionProofs.hashCode()
    result = 31 * result + invalidityProofs.hashCode()
    result = 31 * result + parentAggregationLastBlockTimestamp.hashCode()
    result = 31 * result + parentAggregationLastL1RollingHashMessageNumber.hashCode()
    result = 31 * result + parentAggregationLastL1RollingHash.contentHashCode()
    result = 31 * result + parentAggregationLastFtxNumber.hashCode()
    result = 31 * result + parentAggregationLastFtxRollingHash.contentHashCode()
    result = 31 * result + startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    return result
  }

  override fun toString(): String {
    return "ProofsToAggregate(" +
      "startBlockNumber=$startBlockNumber, " +
      "endBlockNumber=$endBlockNumber, " +
      "startBlockTimestamp=$startBlockTimestamp, " +
      "compressionProofIndexes=$compressionProofIndexes, " +
      "executionProofs=$executionProofs, " +
      "invalidityProofs=$invalidityProofs, " +
      "parentAggregationLastBlockTimestamp=$parentAggregationLastBlockTimestamp, " +
      "parentAggregationLastL1RollingHashMessageNumber=$parentAggregationLastL1RollingHashMessageNumber, " +
      "parentAggregationLastL1RollingHash=${parentAggregationLastL1RollingHash.encodeHex()}, " +
      "parentAggregationLastFtxNumber=$parentAggregationLastFtxNumber, " +
      "parentAggregationLastFtxRollingHash=${parentAggregationLastFtxRollingHash.encodeHex()})"
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
  val parentAggregationFtxNumber: ULong,
  val parentAggregationFtxRollingHash: ByteArray,
  val finalFtxNumber: ULong,
  val finalFtxRollingHash: ByteArray,
  val filteredAddresses: List<ByteArray>,
) : BlockInterval {
  override val startBlockNumber: ULong = firstBlockNumber.toULong()
  override val endBlockNumber: ULong = finalBlockNumber.toULong()

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ProofToFinalize

    if (aggregatedVerifierIndex != other.aggregatedVerifierIndex) return false
    if (firstBlockNumber != other.firstBlockNumber) return false
    if (finalBlockNumber != other.finalBlockNumber) return false
    if (l1RollingHashMessageNumber != other.l1RollingHashMessageNumber) return false
    if (l2MerkleTreesDepth != other.l2MerkleTreesDepth) return false
    if (!aggregatedProof.contentEquals(other.aggregatedProof)) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (!aggregatedProofPublicInput.contentEquals(other.aggregatedProofPublicInput)) return false
    if (!dataHashes.byteArrayListEquals(other.dataHashes)) return false
    if (!dataParentHash.contentEquals(other.dataParentHash)) return false
    if (parentAggregationLastBlockTimestamp != other.parentAggregationLastBlockTimestamp) return false
    if (finalTimestamp != other.finalTimestamp) return false
    if (!l1RollingHash.contentEquals(other.l1RollingHash)) return false
    if (!l2MerkleRoots.byteArrayListEquals(other.l2MerkleRoots)) return false
    if (!l2MessagingBlocksOffsets.contentEquals(other.l2MessagingBlocksOffsets)) return false
    if (parentAggregationFtxNumber != other.parentAggregationFtxNumber) return false
    if (!parentAggregationFtxRollingHash.contentEquals(other.parentAggregationFtxRollingHash)) return false
    if (finalFtxNumber != other.finalFtxNumber) return false
    if (!finalFtxRollingHash.contentEquals(other.finalFtxRollingHash)) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false
    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false

    return true
  }

  override fun hashCode(): Int {
    var result = aggregatedVerifierIndex
    result = 31 * result + firstBlockNumber.hashCode()
    result = 31 * result + finalBlockNumber.hashCode()
    result = 31 * result + l1RollingHashMessageNumber.hashCode()
    result = 31 * result + l2MerkleTreesDepth
    result = 31 * result + aggregatedProof.contentHashCode()
    result = 31 * result + parentStateRootHash.contentHashCode()
    result = 31 * result + aggregatedProofPublicInput.contentHashCode()
    result = 31 * result + dataHashes.byteArrayListHashCode()
    result = 31 * result + dataParentHash.contentHashCode()
    result = 31 * result + parentAggregationLastBlockTimestamp.hashCode()
    result = 31 * result + finalTimestamp.hashCode()
    result = 31 * result + l1RollingHash.contentHashCode()
    result = 31 * result + l2MerkleRoots.byteArrayListHashCode()
    result = 31 * result + l2MessagingBlocksOffsets.contentHashCode()
    result = 31 * result + parentAggregationFtxNumber.hashCode()
    result = 31 * result + parentAggregationFtxRollingHash.contentHashCode()
    result = 31 * result + finalFtxNumber.hashCode()
    result = 31 * result + finalFtxRollingHash.contentHashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    result = 31 * result + startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
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
