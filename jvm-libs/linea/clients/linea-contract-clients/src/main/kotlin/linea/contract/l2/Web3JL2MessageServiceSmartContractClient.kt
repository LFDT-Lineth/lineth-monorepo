package linea.contract.l2

import linea.contract.ContractDeploymentBlockNumberProvider
import linea.contract.EventBasedContractDeploymentBlockNumberProvider
import linea.contract.StaticContractDeploymentBlockNumberProvider
import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import linea.web3j.SmartContractErrors
import linea.web3j.domain.toWeb3j
import linea.web3j.gas.EIP1559GasProvider
import linea.web3j.requestAsync
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.L2MessageService
import net.consensys.linea.contract.Web3JContractAsyncHelper
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.tx.Contract
import org.web3j.tx.gas.StaticGasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.atomic.AtomicReference
import kotlin.random.Random

class Web3JL2MessageServiceSmartContractClient(
  private val web3j: Web3j,
  private val contractAddress: String,
  private val web3jContractHelper: Web3JContractAsyncHelper,
  private val deploymentBlockNumberProvider: ContractDeploymentBlockNumberProvider,
  private val log: Logger = LogManager.getLogger(Web3JL2MessageServiceSmartContractClient::class.java),
) : L2MessageServiceSmartContractClient {
  companion object {
    fun create(
      web3jClient: Web3j,
      ethApiClient: EthApiClient,
      contractAddress: String,
      gasLimit: ULong,
      maxFeePerGasCap: ULong,
      feeHistoryBlockCount: UInt,
      feeHistoryRewardPercentile: Double,
      transactionManager: AsyncFriendlyTransactionManager,
      smartContractErrors: SmartContractErrors,
      smartContractDeploymentBlockNumber: ULong?,
    ): Web3JL2MessageServiceSmartContractClient {
      val gasProvider = EIP1559GasProvider(
        ethApiClient = ethApiClient,
        config = EIP1559GasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          feeHistoryBlockCount = feeHistoryBlockCount,
          feeHistoryRewardPercentile = feeHistoryRewardPercentile,
        ),
      )
      val web3jContractHelper = Web3JContractAsyncHelper(
        contractAddress = contractAddress,
        web3j = web3jClient,
        contractGasProvider = gasProvider,
        transactionManager = transactionManager,
        smartContractErrors = smartContractErrors,
        useEthEstimateGas = true,
      )
      val deploymentBlockNumberProvider = smartContractDeploymentBlockNumber
        ?.let { StaticContractDeploymentBlockNumberProvider(it) }
        ?: EventBasedContractDeploymentBlockNumberProvider(
          ethApiClient = ethApiClient,
          contractAddress = contractAddress,
          log = LogManager.getLogger(Web3JL2MessageServiceSmartContractClient::class.java),
        )

      return Web3JL2MessageServiceSmartContractClient(
        web3j = web3jClient,
        contractAddress = contractAddress,
        web3jContractHelper = web3jContractHelper,
        deploymentBlockNumberProvider = deploymentBlockNumberProvider,
      )
    }

    fun createReadOnly(
      web3jClient: Web3j,
      ethApiClient: EthApiClient,
      contractAddress: String,
      smartContractErrors: SmartContractErrors,
      smartContractDeploymentBlockNumber: ULong?,
    ): L2MessageServiceSmartContractClientReadOnly {
      val unUsedTxManager = AsyncFriendlyTransactionManager(
        web3j = web3jClient,
        credentials = Credentials.create(Random.nextBytes(64).encodeHex()),
        chainId = 1L,
      )
      return create(
        web3jClient = web3jClient,
        ethApiClient = ethApiClient,
        contractAddress = contractAddress,
        gasLimit = 0UL,
        maxFeePerGasCap = 0UL,
        feeHistoryBlockCount = 1u,
        feeHistoryRewardPercentile = 100.0,
        transactionManager = unUsedTxManager,
        smartContractErrors = smartContractErrors,
        smartContractDeploymentBlockNumber = smartContractDeploymentBlockNumber,
      )
    }
  }

  private val fakeCredentials = Credentials.create(ByteArray(32).encodeHex())
  private val smartContractVersionCache = AtomicReference<L2MessageServiceSmartContractVersion>(null)

  private fun <T : Contract> contractClientAtBlock(blockParameter: BlockParameter, contract: Class<T>): T {
    @Suppress("UNCHECKED_CAST")
    return when {
      L2MessageService::class.java.isAssignableFrom(contract) -> L2MessageService.load(
        contractAddress,
        web3j,
        fakeCredentials,
        StaticGasProvider(BigInteger.ZERO, BigInteger.ZERO),
      ).apply {
        this.setDefaultBlockParameter(blockParameter.toWeb3j())
      }

      else -> throw IllegalArgumentException("Unsupported contract type: ${contract::class.java}")
    } as T
  }

  private fun getSmartContractVersion(): SafeFuture<L2MessageServiceSmartContractVersion> {
    return if (smartContractVersionCache.get() == L2MessageServiceSmartContractVersion.V1) {
      // once upgraded, it's not downgraded
      SafeFuture.completedFuture(L2MessageServiceSmartContractVersion.V1)
    } else {
      fetchSmartContractVersion()
        .thenPeek { contractLatestVersion ->
          if (smartContractVersionCache.get() != null &&
            contractLatestVersion != smartContractVersionCache.get()
          ) {
            log.info(
              "L2 Message Service Smart contract upgraded: prevVersion={} upgradedVersion={}",
              smartContractVersionCache.get(),
              contractLatestVersion,
            )
          }
          smartContractVersionCache.set(contractLatestVersion)
        }
    }
  }

  private fun fetchSmartContractVersion(): SafeFuture<L2MessageServiceSmartContractVersion> {
    return contractClientAtBlock(BlockParameter.Tag.LATEST, L2MessageService::class.java)
      .CONTRACT_VERSION()
      .requestAsync { version ->
        when {
          version.startsWith("1") -> L2MessageServiceSmartContractVersion.V1
          else -> throw IllegalStateException("Unsupported contract version: $version")
        }
      }
  }

  override fun getAddress(): String = contractAddress
  override fun getVersion(): SafeFuture<L2MessageServiceSmartContractVersion> = getSmartContractVersion()
  override fun getDeploymentBlock(): SafeFuture<ULong> {
    return deploymentBlockNumberProvider()
  }

  override fun getLastAnchoredL1MessageNumber(block: BlockParameter): SafeFuture<ULong> {
    return contractClientAtBlock(block, L2MessageService::class.java)
      .lastAnchoredL1MessageNumber()
      .requestAsync { it.toULong() }
  }

  override fun getRollingHashByL1MessageNumber(block: BlockParameter, l1MessageNumber: ULong): SafeFuture<ByteArray> {
    return contractClientAtBlock(block, L2MessageService::class.java)
      .l1RollingHashes(l1MessageNumber.toBigInteger())
      .requestAsync { it }
  }

  override fun anchorL1L2MessageHashes(
    messageHashes: List<ByteArray>,
    startingMessageNumber: ULong,
    finalMessageNumber: ULong,
    finalRollingHash: ByteArray,
  ): SafeFuture<String> {
    return anchorL1L2MessageHashesV2(
      messageHashes = messageHashes,
      startingMessageNumber = startingMessageNumber.toBigInteger(),
      finalMessageNumber = finalMessageNumber.toBigInteger(),
      finalRollingHash = finalRollingHash,
    )
  }

  private fun anchorL1L2MessageHashesV2(
    messageHashes: List<ByteArray>,
    startingMessageNumber: BigInteger,
    finalMessageNumber: BigInteger,
    finalRollingHash: ByteArray,
  ): SafeFuture<String> {
    val function = buildAnchorL1L2MessageHashesV1(
      messageHashes = messageHashes,
      startingMessageNumber = startingMessageNumber,
      finalMessageNumber = finalMessageNumber,
      finalRollingHash = finalRollingHash,
    )

    return web3jContractHelper
      .transactionManager
      .resetNonce(blockParameter = BlockParameter.Tag.LATEST)
      .thenCompose {
        web3jContractHelper
          .sendTransactionAfterEthCallAsync(
            function = function,
            weiValue = BigInteger.ZERO,
            gasPriceCaps = null,
          )
      }
      .thenApply { response ->
        response.transactionHash
      }
  }
}
