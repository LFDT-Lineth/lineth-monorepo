package lineth.contract.l1

import lineth.EthLogsSearcher
import lineth.contract.LinethRollupV6
import lineth.contract.Web3JContractAsyncHelper
import lineth.domain.BlobRecord
import lineth.domain.ProofToFinalize
import lineth.domain.gas.GasPriceCaps
import lineth.domain.toBlockParameter
import lineth.kotlin.toULong
import lineth.web3j.SmartContractErrors
import lineth.web3j.transactionmanager.AsyncFriendlyTransactionManager
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
  ethLogsSearcher: EthLogsSearcher,
  l1EventSearchMaxBlockRange: UInt = 10_000u,
  private val log: Logger = LogManager.getLogger(Web3JLinethRollupSmartContractClient::class.java),
) : Web3JLinethRollupSmartContractClientReadOnly(
  contractAddress = contractAddress,
  web3j = web3j,
  ethLogsSearcher = ethLogsSearcher,
  l1EventSearchMaxBlockRange = l1EventSearchMaxBlockRange,
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
      ethLogsSearcher: EthLogsSearcher,
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
        ethLogsSearcher = ethLogsSearcher,
      )
    }

    fun load(
      contractAddress: String,
      web3j: Web3j,
      credentials: Credentials,
      contractGasProvider: ContractGasProvider,
      smartContractErrors: SmartContractErrors,
      ethLogsSearcher: EthLogsSearcher,
      useEthEstimateGas: Boolean,
    ): Web3JLinethRollupSmartContractClient {
      return load(
        contractAddress,
        web3j,
        // chainId will default -1, which will create legacy transactions
        AsyncFriendlyTransactionManager(web3j, credentials),
        contractGasProvider,
        smartContractErrors,
        ethLogsSearcher,
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

  private fun ensureMinVersion(
    minVersion: LinethRollupContractVersion,
  ): SafeFuture<LinethRollupContractVersion> {
    return getVersion()
      .thenCompose { version ->
        if (version < minVersion) {
          SafeFuture.failedFuture(
            RuntimeException("Contract version=$version is lower than minimum required version=$minVersion"),
          )
        } else {
          SafeFuture.completedFuture(version)
        }
      }
  }

  override fun submitBlobsV9(
    blobData: BlobsSubmissionV9,
    gasPriceCaps: GasPriceCaps?,
    preflightWithEthCall: Boolean,
  ): SafeFuture<String> {
    return ensureMinVersion(LinethRollupContractVersion.V9)
      .thenApply {
        FunctionBuildersV9.buildSubmitBlobsFunctionV9(blobData)
      }
      .thenCompose { function ->
        if (preflightWithEthCall) {
          web3jContractHelper.executeBlobEthCall(
            function = function,
            blobs = blobData.blobs,
            gasPriceCaps = gasPriceCaps,
          )
        } else {
          SafeFuture.completedFuture(null)
        }.thenCompose {
          web3jContractHelper.sendBlobCarryingTransactionAndGetTxHash(
            function = function,
            blobs = blobData.blobs,
            gasPriceCaps = gasPriceCaps,
          )
        }
      }
  }

  override fun finalizeBlocksV9(
    data: FinalizationDataV9,
    gasPriceCaps: GasPriceCaps?,
    preflightWithEthCall: Boolean,
  ): SafeFuture<String> {
    return ensureMinVersion(LinethRollupContractVersion.V9)
      .thenApply {
        FunctionBuildersV9.buildFinalizeBlocksFunctionV9(data)
      }
      .thenCompose { function ->
        if (preflightWithEthCall) {
          web3jContractHelper.sendTransactionAfterEthCallAsync(function, BigInteger.ZERO, gasPriceCaps)
        } else {
          web3jContractHelper.sendTransactionAsync(function, BigInteger.ZERO, gasPriceCaps)
        }
          .thenApply { result -> result.transactionHash }
      }
  }
}
