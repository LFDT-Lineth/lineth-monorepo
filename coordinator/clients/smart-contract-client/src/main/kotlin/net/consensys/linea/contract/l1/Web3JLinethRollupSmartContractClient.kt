package net.consensys.linea.contract.l1

import linea.contract.LinethRollupV6
import linea.contract.l1.BlockAndNonce
import linea.contract.l1.LinethRollupSmartContractClient
import linea.contract.l1.Web3JLinethRollupSmartContractClientReadOnly
import linea.domain.BlobRecord
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.ProofToFinalize
import linea.domain.gas.GasPriceCaps
import linea.ethapi.EthLogsClient
import linea.kotlin.toULong
import linea.web3j.SmartContractErrors
import linea.web3j.ethapi.createEthApiClient
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.Web3JContractAsyncHelper
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.tx.gas.ContractGasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class Web3JLinethRollupSmartContractClient internal constructor(
  contractAddress: String,
  web3j: Web3j,
  private val transactionManager: AsyncFriendlyTransactionManager,
  private val web3jContractHelper: Web3JContractAsyncHelper,
  private val web3jLineaClient: LinethRollupV6,
  ethLogsClient: EthLogsClient = createEthApiClient(web3j),
  private val log: Logger = LogManager.getLogger(Web3JLinethRollupSmartContractClient::class.java),
) : Web3JLinethRollupSmartContractClientReadOnly(
  contractAddress = contractAddress,
  web3j = web3j,
  ethLogsClient = ethLogsClient,
  log = log,
),
  LinethRollupSmartContractClient {
  companion object {
    fun load(
      contractAddress: String,
      web3j: Web3j,
      transactionManager: AsyncFriendlyTransactionManager,
      contractGasProvider: ContractGasProvider,
      smartContractErrors: SmartContractErrors,
      useEthEstimateGas: Boolean = false,
    ): Web3JLinethRollupSmartContractClient {
      val web3JContractAsyncHelper =
        Web3JContractAsyncHelper(
          contractAddress = contractAddress,
          web3j = web3j,
          transactionManager = transactionManager,
          contractGasProvider = contractGasProvider,
          smartContractErrors = smartContractErrors,
          useEthEstimateGas = useEthEstimateGas,
        )
      val linethRollupEnhancedWrapper =
        LinethRollupEnhancedWrapper(
          contractAddress = contractAddress,
          web3j = web3j,
          transactionManager = transactionManager,
          contractGasProvider = contractGasProvider,
          web3jContractHelper = web3JContractAsyncHelper,
        )
      return Web3JLinethRollupSmartContractClient(
        contractAddress = contractAddress,
        web3j = web3j,
        transactionManager = transactionManager,
        web3jContractHelper = web3JContractAsyncHelper,
        web3jLineaClient = linethRollupEnhancedWrapper,
      )
    }

    fun load(
      contractAddress: String,
      web3j: Web3j,
      credentials: Credentials,
      contractGasProvider: ContractGasProvider,
      smartContractErrors: SmartContractErrors,
      useEthEstimateGas: Boolean,
    ): Web3JLinethRollupSmartContractClient {
      return load(
        contractAddress,
        web3j,
        // chainId will default -1, which will create legacy transactions
        AsyncFriendlyTransactionManager(web3j, credentials),
        contractGasProvider,
        smartContractErrors,
        useEthEstimateGas,
      )
    }
  }

  override fun currentNonce(): ULong {
    return transactionManager.currentNonce().toULong()
  }

  private fun resetNonce(blockNumber: BigInteger): SafeFuture<ULong> {
    return transactionManager
      .resetNonce(blockNumber.toBlockParameter())
      .thenApply { currentNonce() }
  }

  override fun updateNonceAndReferenceBlockToLastL1Block(): SafeFuture<BlockAndNonce> {
    return web3jContractHelper.getCurrentBlock()
      .thenCompose { blockNumber ->
        web3jLineaClient.setDefaultBlockParameter(DefaultBlockParameter.valueOf(blockNumber))
        resetNonce(blockNumber)
          .thenApply { currentNonce -> BlockAndNonce(blockNumber.toULong(), currentNonce) }
      }
  }

  /**
   * Sends EIP4844 blob carrying transaction to the smart contract.
   * Uses SMC `submitBlobs` function that supports multiple blobs per call.
   */
  override fun submitBlobs(blobs: List<BlobRecord>, gasPriceCaps: GasPriceCaps?): SafeFuture<String> {
    return getVersion()
      .thenCompose { version ->
        val function = Web3JLinethRollupFunctionBuilders.buildSubmitBlobsFunction(version, blobs)
        web3jContractHelper.sendBlobCarryingTransactionAndGetTxHash(
          function = function,
          blobs = blobs.map { it.blobCompressionProof!!.compressedData },
          gasPriceCaps = gasPriceCaps,
        )
      }
  }

  override fun submitBlobsEthCall(blobs: List<BlobRecord>, gasPriceCaps: GasPriceCaps?): SafeFuture<String?> {
    return getVersion()
      .thenCompose { version ->
        val function = Web3JLinethRollupFunctionBuilders.buildSubmitBlobsFunction(version, blobs)
        web3jContractHelper.executeBlobEthCall(
          function = function,
          blobs = blobs.map { it.blobCompressionProof!!.compressedData },
          gasPriceCaps = gasPriceCaps,
        )
      }
  }

  override fun finalizeBlocks(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
    gasPriceCaps: GasPriceCaps?,
  ): SafeFuture<String> {
    return getVersion()
      .thenCompose { version ->
        val function =
          Web3JLinethRollupFunctionBuilders.buildFinalizeBlocksFunction(
            version,
            aggregation,
            aggregationLastBlob,
            parentL1RollingHash,
            parentL1RollingHashMessageNumber,
          )
        web3jContractHelper
          .sendTransactionAsync(function, BigInteger.ZERO, gasPriceCaps)
          .thenApply { result -> result.transactionHash }
      }
  }

  override fun finalizeBlocksEthCall(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): SafeFuture<String?> {
    return getVersion()
      .thenCompose { version ->
        val function =
          Web3JLinethRollupFunctionBuilders.buildFinalizeBlocksFunction(
            version,
            aggregation,
            aggregationLastBlob,
            parentL1RollingHash,
            parentL1RollingHashMessageNumber,
          )
        web3jContractHelper.executeEthCall(function)
      }
  }

  override fun finalizeBlocksAfterEthCall(
    aggregation: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
    gasPriceCaps: GasPriceCaps?,
  ): SafeFuture<String> {
    return getVersion()
      .thenCompose { version ->
        val function =
          Web3JLinethRollupFunctionBuilders.buildFinalizeBlocksFunction(
            version,
            aggregation,
            aggregationLastBlob,
            parentL1RollingHash,
            parentL1RollingHashMessageNumber,
          )
        web3jContractHelper.sendTransactionAfterEthCallAsync(function, BigInteger.ZERO, gasPriceCaps)
          .thenApply { result -> result.transactionHash }
      }
  }
}
