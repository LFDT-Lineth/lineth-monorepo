package net.consensys.zkevm.coordinator.clients.smartcontract

import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.contract.l1.LineaSmartContractClientReadOnly
import linea.contract.l1.LineaValidiumSmartContractClientReadOnly
import linea.domain.gas.GasPriceCaps
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.ProofToFinalize
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class BlockAndNonce(
  val blockNumber: ULong,
  val nonce: ULong,
)

interface LineaSmartContractClient : LineaSmartContractClientReadOnly {
  fun currentNonce(): ULong

  /**
   * Fetches LATEST block from L1, correspondent nonce at that block
   * and sets internal state to those
   */
  fun updateNonceAndReferenceBlockToLastL1Block(): SafeFuture<BlockAndNonce>

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
