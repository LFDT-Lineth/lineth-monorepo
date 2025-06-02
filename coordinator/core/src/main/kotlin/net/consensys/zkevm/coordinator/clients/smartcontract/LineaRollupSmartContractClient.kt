package net.consensys.zkevm.coordinator.clients.smartcontract

import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.domain.gas.GasPriceCaps
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.ProofToFinalize
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class BlockAndNonce(
  val blockNumber: ULong,
  val nonce: ULong,
)

interface LineaRollupSmartContractClient : LineaRollupSmartContractClientReadOnly {

  fun currentNonce(): ULong

  /**
   * Fetches LATEST block from L1, correspondent nonce at that block
   * and sets internal state to those
   */
  fun updateNonceAndReferenceBlockToLastL1Block(): SafeFuture<BlockAndNonce>

  /**
   *  Simulates the sending of a list of blobs to the smart contract, with EIP4844 transaction.
   */
  fun submitBlobsEthCall(
    blobs: List<BlobRecord>,
    gasPriceCaps: GasPriceCaps?,
  ): SafeFuture<String?>

  fun finalizeBlocksEthCall(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): SafeFuture<String?>

  /**
   * Submit a list of blobs to the smart contract, with EIP4844 transaction
   */
  fun submitBlobs(
    blobs: List<BlobRecord>,
    gasPriceCaps: GasPriceCaps?,
  ): SafeFuture<String>

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

interface LineaGenesisStateProvider {
  val stateRootHash: ByteArray
  val shnarf: ByteArray
}
