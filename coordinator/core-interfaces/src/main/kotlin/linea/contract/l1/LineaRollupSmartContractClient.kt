package linea.contract.l1

import linea.domain.BlobRecord
import linea.domain.ProofToFinalize
import linea.domain.gas.GasPriceCaps
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
)

data class ShnarfDataV9(
  val parentShnarf: ByteArray,
  val snarkHash: ByteArray,
  val finalStateRootHash: ByteArray,
  val dataEvaluationPoint: ByteArray,
  val dataEvaluationClaim: ByteArray,
)

data class FinalizationDataV9(
  val aggregatedProof: ByteArray,
  val proofType: Int,
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
  val l2MerkleTreesDepth: Int,
  val lastFinalizedForcedTransactionNumber: ULong,
  val finalForcedTransactionNumber: ULong,
  val lastFinalizedForcedTransactionRollingHash: ByteArray,
  val finalBlockHash: ByteArray,
  val finalBlobHash: ByteArray,
  val l2MerkleRoots: List<ByteArray>,
  val filteredAddresses: List<ByteArray>,
  val verifierKeys: List<ByteArray>,
  val l2MessagingBlocksOffsets: ByteArray,
)

interface LineaSmartContractClient : LineaSmartContractClientReadOnly {
  fun currentNonce(): ULong

  /**
   * Fetches LATEST block from L1, correspondent nonce at that block
   * and sets internal state to those
   */
  fun updateNonceAndReferenceBlockToLastL1Block(): SafeFuture<BlockAndNonce>

  // TODO: not used, shall be removed
  fun finalizeBlocksEthCall(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): SafeFuture<String?>

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

interface LineaRollupSmartContractClient :
  LineaRollupSmartContractClientReadOnly,
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
   */
  fun submitBlobsV9(
    blobData: BlobsSubmissionV9,
    gasPriceCaps: GasPriceCaps?,
    preflightWithEthCall: Boolean = true,
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
