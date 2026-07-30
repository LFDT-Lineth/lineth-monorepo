package linea.contract.l1

import linea.domain.BlobRecord
import linea.domain.ProofToFinalize
import linea.domain.gas.GasPriceCaps
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

data class BlockAndNonce(
  val blockNumber: ULong,
  val nonce: ULong,
)

data class BlobsSubmissionV9(
  val blobs: List<ByteArray>,
  val blobFinalBlockHashes: List<ByteArray>,
  val parentShnarf: ByteArray,
  val finalBlobShnarf: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BlobsSubmissionV9

    if (!blobs.byteArrayListEquals(other.blobs)) return false
    if (!blobFinalBlockHashes.byteArrayListEquals(other.blobFinalBlockHashes)) return false
    if (!parentShnarf.contentEquals(other.parentShnarf)) return false
    if (!finalBlobShnarf.contentEquals(other.finalBlobShnarf)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blobs.byteArrayListHashCode()
    result = 31 * result + blobFinalBlockHashes.byteArrayListHashCode()
    result = 31 * result + parentShnarf.contentHashCode()
    result = 31 * result + finalBlobShnarf.contentHashCode()
    return result
  }
}

data class ShnarfDataV9(
  val parentShnarf: ByteArray,
  val snarkHash: ByteArray,
  val finalStateRootHash: ByteArray,
  val dataEvaluationPoint: ByteArray,
  val dataEvaluationClaim: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ShnarfDataV9

    if (!parentShnarf.contentEquals(other.parentShnarf)) return false
    if (!snarkHash.contentEquals(other.snarkHash)) return false
    if (!finalStateRootHash.contentEquals(other.finalStateRootHash)) return false
    if (!dataEvaluationPoint.contentEquals(other.dataEvaluationPoint)) return false
    if (!dataEvaluationClaim.contentEquals(other.dataEvaluationClaim)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = parentShnarf.contentHashCode()
    result = 31 * result + snarkHash.contentHashCode()
    result = 31 * result + finalStateRootHash.contentHashCode()
    result = 31 * result + dataEvaluationPoint.contentHashCode()
    result = 31 * result + dataEvaluationClaim.contentHashCode()
    return result
  }
}

data class FinalizationDataV9(
  val aggregatedProof: ByteArray,
  val proofType: UInt,
  val parentStateRootHash: ByteArray,
  val parentBlockHash: ByteArray,
  val endBlockNumber: ULong,
  val shnarfData: ShnarfDataV9,
  val lastFinalizedTimestamp: Instant,
  val finalTimestamp: Instant,
  val lastFinalizedL1RollingHash: ByteArray,
  val l1RollingHash: ByteArray,
  val lastFinalizedL1RollingHashMessageNumber: ULong,
  val l1RollingHashMessageNumber: ULong,
  val l2MerkleTreesDepth: UInt,
  val lastFinalizedForcedTransactionNumber: ULong,
  val finalForcedTransactionNumber: ULong,
  val lastFinalizedForcedTransactionRollingHash: ByteArray,
  val finalBlockHash: ByteArray,
  val finalBlobHash: ByteArray,
  val l2MerkleRoots: List<ByteArray>,
  val filteredAddresses: List<ByteArray>,
  val verifierKeys: List<ByteArray>,
  val l2MessagingBlocksOffsets: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as FinalizationDataV9

    if (proofType != other.proofType) return false
    if (l2MerkleTreesDepth != other.l2MerkleTreesDepth) return false
    if (!aggregatedProof.contentEquals(other.aggregatedProof)) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (!parentBlockHash.contentEquals(other.parentBlockHash)) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (shnarfData != other.shnarfData) return false
    if (lastFinalizedTimestamp != other.lastFinalizedTimestamp) return false
    if (finalTimestamp != other.finalTimestamp) return false
    if (!lastFinalizedL1RollingHash.contentEquals(other.lastFinalizedL1RollingHash)) return false
    if (!l1RollingHash.contentEquals(other.l1RollingHash)) return false
    if (lastFinalizedL1RollingHashMessageNumber != other.lastFinalizedL1RollingHashMessageNumber) return false
    if (l1RollingHashMessageNumber != other.l1RollingHashMessageNumber) return false
    if (lastFinalizedForcedTransactionNumber != other.lastFinalizedForcedTransactionNumber) return false
    if (finalForcedTransactionNumber != other.finalForcedTransactionNumber) return false
    if (!lastFinalizedForcedTransactionRollingHash.contentEquals(
        other.lastFinalizedForcedTransactionRollingHash,
      )
    ) {
      return false
    }
    if (!finalBlockHash.contentEquals(other.finalBlockHash)) return false
    if (!finalBlobHash.contentEquals(other.finalBlobHash)) return false
    if (!l2MerkleRoots.byteArrayListEquals(other.l2MerkleRoots)) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false
    if (!verifierKeys.byteArrayListEquals(other.verifierKeys)) return false
    if (!l2MessagingBlocksOffsets.contentEquals(other.l2MessagingBlocksOffsets)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = proofType.toInt()
    result = 31 * result + l2MerkleTreesDepth.toInt()
    result = 31 * result + aggregatedProof.contentHashCode()
    result = 31 * result + parentStateRootHash.contentHashCode()
    result = 31 * result + parentBlockHash.contentHashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + shnarfData.hashCode()
    result = 31 * result + lastFinalizedTimestamp.hashCode()
    result = 31 * result + finalTimestamp.hashCode()
    result = 31 * result + lastFinalizedL1RollingHash.contentHashCode()
    result = 31 * result + l1RollingHash.contentHashCode()
    result = 31 * result + lastFinalizedL1RollingHashMessageNumber.hashCode()
    result = 31 * result + l1RollingHashMessageNumber.hashCode()
    result = 31 * result + lastFinalizedForcedTransactionNumber.hashCode()
    result = 31 * result + finalForcedTransactionNumber.hashCode()
    result = 31 * result + lastFinalizedForcedTransactionRollingHash.contentHashCode()
    result = 31 * result + finalBlockHash.contentHashCode()
    result = 31 * result + finalBlobHash.contentHashCode()
    result = 31 * result + l2MerkleRoots.byteArrayListHashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    result = 31 * result + verifierKeys.byteArrayListHashCode()
    result = 31 * result + l2MessagingBlocksOffsets.contentHashCode()
    return result
  }
}

interface LineaSmartContractClient : LineaSmartContractClientReadOnly {
  fun currentNonce(): ULong

  /**
   * Fetches LATEST block from L1, correspondent nonce at that block
   * and sets internal state to those
   */
  fun updateNonceAndReferenceBlockToLastL1Block(): SafeFuture<BlockAndNonce>

  fun finalizeBlocks(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
    gasPriceCaps: GasPriceCaps?,
  ): SafeFuture<String>

  fun finalizeBlocksAfterEthCall(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
    gasPriceCaps: GasPriceCaps?,
  ): SafeFuture<String>
}

interface LinethRollupSmartContractClient :
  LinethRollupSmartContractClientReadOnly,
  LineaSmartContractClient {
  /**
   *  Simulates the sending of a list of blobs to the smart contract, with EIP4844 transaction.
   */
  fun submitBlobsEthCall(blobs: List<BlobRecord>, gasPriceCaps: GasPriceCaps?): SafeFuture<String?>

  /**
   * Submit a list of blobs to the smart contract, with EIP4844 transaction
   */
  fun submitBlobs(blobs: List<BlobRecord>, gasPriceCaps: GasPriceCaps?): SafeFuture<String>

  /**
   * Submits blocks for V9 (RISC-V arithmetization)
   * @param preflightWithEthCall when true will call eth_call to prevalidate the transaction does not revert
   * @param onlyEthCall when true will only call eth_call without doing the actual transaction submission
   */
  fun submitBlobsV9(
    blobData: BlobsSubmissionV9,
    gasPriceCaps: GasPriceCaps?,
    preflightWithEthCall: Boolean = true,
    onlyEthCall: Boolean = false,
  ): SafeFuture<String>

  /**
   * Finalizes blocks for V9 (RISC-V arithmetization)
   * @param preflightWithEthCall when true will call eth_call to prevalidate the transaction does not revert
   */
  fun finalizeBlocksV9(
    data: FinalizationDataV9,
    gasPriceCaps: GasPriceCaps?,
    preflightWithEthCall: Boolean = true,
  ): SafeFuture<String>
}

interface LineaValidiumSmartContractClient :
  LineaValidiumSmartContractClientReadOnly,
  LineaSmartContractClient {
  /**
   *  Simulates the sending of a list of blobs to the smart contract, with EIP4844 transaction.
   */
  fun acceptShnarfDataEthCall(blobs: List<BlobRecord>, gasPriceCaps: GasPriceCaps?): SafeFuture<String?>

  /**
   * Submit a list of blobs to the smart contract, with EIP4844 transaction
   */
  fun acceptShnarfData(blobs: List<BlobRecord>, gasPriceCaps: GasPriceCaps?): SafeFuture<String>
}
