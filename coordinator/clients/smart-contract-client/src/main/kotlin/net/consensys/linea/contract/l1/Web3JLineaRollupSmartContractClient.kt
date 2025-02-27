package net.consensys.linea.contract.l1

import build.linea.contract.LineaRollupV6
import build.linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import linea.kotlin.toULong
import net.consensys.linea.contract.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.Web3JContractAsyncHelper
import net.consensys.linea.contract.throwExceptionIfJsonRpcErrorReturned
import net.consensys.linea.web3j.SmartContractErrors
import net.consensys.zkevm.coordinator.clients.smartcontract.BlockAndNonce
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCaps
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.tx.gas.ContractGasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class Web3JLineaRollupSmartContractClient internal constructor(
  contractAddress: String,
  web3j: Web3j,
  private val transactionManager: AsyncFriendlyTransactionManager,
  private val web3jContractHelper: Web3JContractAsyncHelper,
  private val web3jLineaClient: LineaRollupV6,
  private val log: Logger = LogManager.getLogger(Web3JLineaRollupSmartContractClient::class.java)
) : Web3JLineaRollupSmartContractClientReadOnly(
  contractAddress = contractAddress,
  web3j = web3j,
  log = log
),
  LineaRollupSmartContractClient {

  companion object {
    fun load(
      contractAddress: String,
      web3j: Web3j,
      transactionManager: AsyncFriendlyTransactionManager,
      contractGasProvider: ContractGasProvider,
      smartContractErrors: SmartContractErrors,
      useEthEstimateGas: Boolean = false
    ): Web3JLineaRollupSmartContractClient {
      val web3JContractAsyncHelper = Web3JContractAsyncHelper(
        contractAddress = contractAddress,
        web3j = web3j,
        transactionManager = transactionManager,
        contractGasProvider = contractGasProvider,
        smartContractErrors = smartContractErrors,
        useEthEstimateGas = useEthEstimateGas
      )
      val lineaRollupEnhancedWrapper = LineaRollupEnhancedWrapper(
        contractAddress = contractAddress,
        web3j = web3j,
        transactionManager = transactionManager,
        contractGasProvider = contractGasProvider,
        web3jContractHelper = web3JContractAsyncHelper
      )
      return Web3JLineaRollupSmartContractClient(
        contractAddress = contractAddress,
        web3j = web3j,
        transactionManager = transactionManager,
        web3jContractHelper = web3JContractAsyncHelper,
        web3jLineaClient = lineaRollupEnhancedWrapper
      )
    }

    fun load(
      contractAddress: String,
      web3j: Web3j,
      credentials: Credentials,
      contractGasProvider: ContractGasProvider,
      smartContractErrors: SmartContractErrors,
      useEthEstimateGas: Boolean
    ): Web3JLineaRollupSmartContractClient {
      return load(
        contractAddress,
        web3j,
        // chainId will default -1, which will create legacy transactions
        AsyncFriendlyTransactionManager(web3j, credentials),
        contractGasProvider,
        smartContractErrors,
        useEthEstimateGas
      )
    }
  }

  override fun currentNonce(): ULong {
    return transactionManager.currentNonce().toULong()
  }

  private fun resetNonce(blockNumber: BigInteger?): SafeFuture<ULong> {
    return transactionManager
      .resetNonce(blockNumber)
      .thenApply { currentNonce() }
  }

  override fun updateNonceAndReferenceBlockToLastL1Block(): SafeFuture<BlockAndNonce> {
    return web3jContractHelper.getCurrentBlock()
      .thenCompose { blockNumber ->
        web3jLineaClient.setDefaultBlockParameter(DefaultBlockParameter.valueOf(blockNumber))
        resetNonce(blockNumber)
          .thenApply { BlockAndNonce(blockNumber.toULong(), currentNonce()) }
      }
  }

  /**
   * Sends EIP4844 blob carrying transaction to the smart contract.
   * Uses SMC `submitBlobs` function that supports multiple blobs per call.
   */
  override fun submitBlobs(
    blobs: List<BlobRecord>,
    gasPriceCaps: GasPriceCaps?
  ): SafeFuture<String> {
    return getVersion()
      .thenCompose { version ->
        val function = buildSubmitBlobsFunction(version, blobs)
        web3jContractHelper.sendBlobCarryingTransactionAndGetTxHash(
          function = function,
          blobs = blobs.map { it.blobCompressionProof!!.compressedData },
          gasPriceCaps = gasPriceCaps
        )
      }
  }

  override fun submitBlobsEthCall(
    blobs: List<BlobRecord>,
    gasPriceCaps: GasPriceCaps?
  ): SafeFuture<String?> {
    return getVersion()
      .thenCompose { version ->
        val function = buildSubmitBlobsFunction(version, blobs)
        web3jContractHelper.executeBlobEthCall(function, blobs, gasPriceCaps)
      }
  }

  override fun finalizeBlocks(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
    gasPriceCaps: GasPriceCaps?
  ): SafeFuture<String> {
    return getVersion()
      .thenCompose { version ->
        val function = buildFinalizeBlocksFunction(
          version,
          aggregation,
          aggregationLastBlob,
          parentL1RollingHash,
          parentL1RollingHashMessageNumber
        )
        web3jContractHelper.sendTransactionAsync(function, BigInteger.ZERO, gasPriceCaps)
          .thenApply { result ->
            throwExceptionIfJsonRpcErrorReturned("eth_sendRawTransaction", result)
            result.transactionHash
          }
      }
  }

  override fun finalizeBlocksEthCall(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long
  ): SafeFuture<String?> {
    return getVersion()
      .thenCompose { version ->
        val function = buildFinalizeBlocksFunction(
          version,
          aggregation,
          aggregationLastBlob,
          parentL1RollingHash,
          parentL1RollingHashMessageNumber
        )
        web3jContractHelper.executeEthCall(function)
      }
  }

  override fun finalizeBlocksAfterEthCall(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
    gasPriceCaps: GasPriceCaps?
  ): SafeFuture<String> {
    return getVersion()
      .thenCompose { version ->
        val function = buildFinalizeBlocksFunction(
          version,
          aggregation,
          aggregationLastBlob,
          parentL1RollingHash,
          parentL1RollingHashMessageNumber
        )
        web3jContractHelper.sendTransactionAfterEthCallAsync(function, BigInteger.ZERO, gasPriceCaps)
          .thenApply { result ->
            throwExceptionIfJsonRpcErrorReturned("eth_sendRawTransaction", result)
            result.transactionHash
          }
      }
  }
}
