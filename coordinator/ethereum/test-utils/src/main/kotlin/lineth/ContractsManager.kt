package lineth

import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.ExperimentalHoplite
import com.sksamuel.hoplite.addFileSource
import io.vertx.core.Vertx
import lineth.contract.l1.LinethRollupContractVersion
import lineth.contract.l1.LinethRollupSmartContractClient
import lineth.contract.l1.Web3JLinethRollupSmartContractClient
import lineth.ethapi.EthLogsSearcherImpl
import lineth.kotlin.gwei
import lineth.testing.filesystem.findPathTo
import lineth.web3j.SmartContractErrors
import lineth.web3j.gas.StaticGasProvider
import lineth.web3j.transactionmanager.AsyncFriendlyTransactionManager
import org.slf4j.LoggerFactory
import org.web3j.tx.gas.ContractEIP1559GasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class LinethRollupDeploymentResult(
  val contractAddress: String,
  val contractDeploymentAccount: Account,
  val contractDeploymentBlockNumber: ULong,
  val rollupOperators: List<AccountTransactionManager>,
  val rollupOperatorClient: LinethRollupSmartContractClient,
) {
  val rollupOperator: AccountTransactionManager
    get() = rollupOperators.first()
}

data class L2MessageServiceDeploymentResult(
  val contractAddress: String,
  val contractDeploymentBlockNumber: ULong,
  val anchorerOperator: AccountTransactionManager,
)

data class ContactsDeploymentResult(
  val linethRollup: LinethRollupDeploymentResult,
  val l2MessageService: L2MessageServiceDeploymentResult,
)

interface ContractsManager {
  /**
   * Deploys a linea rollup contract with specified number of operators.
   * Operator accounts are generated on the fly and funded from whale account in genesis file.
   */
  fun deployLinethRollup(
    numberOfOperators: Int = 1,
    contractVersion: LinethRollupContractVersion,
  ): SafeFuture<LinethRollupDeploymentResult>

  fun deployL2MessageService(): SafeFuture<L2MessageServiceDeploymentResult>

  fun deployRollupAndL2MessageService(
    dataCompressionAndProofAggregationMigrationBlock: ULong = 1000UL,
    numberOfOperators: Int = 1,
    l1ContractVersion: LinethRollupContractVersion = LinethRollupContractVersion.V6,
  ): SafeFuture<ContactsDeploymentResult>

  fun connectToLinethRollupContract(
    contractAddress: String,
    transactionManager: AsyncFriendlyTransactionManager,
    gasProvider: ContractEIP1559GasProvider = StaticGasProvider(
      L1AccountManager.chainId,
      maxFeePerGas = 55UL.gwei,
      maxPriorityFeePerGas = 50UL.gwei,
      maxFeePerBlobGas = 1_000UL.gwei,
      gasLimit = 1_000_000uL,
    ),
    smartContractErrors: SmartContractErrors? = null,
  ): LinethRollupSmartContractClient

  companion object {
    fun get(): ContractsManager = MakeFileDelegatedContractsManager
  }
}

object MakeFileDelegatedContractsManager : ContractsManager {
  private val log = LoggerFactory.getLogger(MakeFileDelegatedContractsManager::class.java)
  private val vertx: Vertx = Vertx.vertx()

  @OptIn(ExperimentalHoplite::class)
  val linethRollupContractErrors = findPathTo("config")!!
    .resolve("common/smart-contract-errors.toml")
    .let { filePath ->
      data class ErrorsFile(val smartContractErrors: Map<String, String>)
      ConfigLoaderBuilder
        .default()
        .withExplicitSealedTypes()
        .addFileSource(filePath.toAbsolutePath().toString())
        .build()
        .loadConfigOrThrow<ErrorsFile>()
        .smartContractErrors
    }

  override fun deployLinethRollup(
    numberOfOperators: Int,
    contractVersion: LinethRollupContractVersion,
  ): SafeFuture<LinethRollupDeploymentResult> {
    val newAccounts = L1AccountManager.generateAccounts(numberOfOperators)
    val contractDeploymentAccount = newAccounts.first()
    val operatorsAccounts = newAccounts.drop(1)
    log.debug(
      "going deploy LinethRollup: deployerAccount={} rollupOperators={}",
      contractDeploymentAccount.address,
      operatorsAccounts.map { it.address },
    )
    val future = makeDeployLinethRollup(
      deploymentPrivateKey = contractDeploymentAccount.privateKey,
      operatorsAddresses = operatorsAccounts.map { it.address },
      contractVersion = contractVersion,
    )
      .thenApply { deploymentResult ->
        log.debug(
          "LinethRollup deployed: address={} blockNumber={} deployerAccount={} " +
            "rollupOperators={}",
          deploymentResult.address,
          deploymentResult.blockNumber,
          contractDeploymentAccount.address,
          operatorsAccounts.map { it.address },
        )
        val accountsTxManagers = operatorsAccounts.map {
          AccountTransactionManager(it, L1AccountManager.getTransactionManager(it))
        }

        val rollupOperatorClient = connectToLinethRollupContract(
          deploymentResult.address,
          accountsTxManagers.first().txManager,
          smartContractErrors = linethRollupContractErrors,
        )
        LinethRollupDeploymentResult(
          contractAddress = deploymentResult.address,
          contractDeploymentAccount = contractDeploymentAccount,
          contractDeploymentBlockNumber = deploymentResult.blockNumber.toULong(),
          rollupOperators = accountsTxManagers,
          rollupOperatorClient = rollupOperatorClient,
        )
      }
    return future
  }

  override fun deployL2MessageService(): SafeFuture<L2MessageServiceDeploymentResult> {
    val (deployerAccount, anchorerAccount) = L2AccountManager.generateAccounts(2)
    return makeDeployL2MessageService(
      deploymentPrivateKey = deployerAccount.privateKey,
      anchorOperatorAddresses = anchorerAccount.address,
    )
      .thenApply {
        L2MessageServiceDeploymentResult(
          contractAddress = it.address,
          contractDeploymentBlockNumber = it.blockNumber.toULong(),
          anchorerOperator = AccountTransactionManager(
            account = anchorerAccount,
            txManager = L2AccountManager.getTransactionManager(anchorerAccount),
          ),
        )
      }
  }

  override fun deployRollupAndL2MessageService(
    dataCompressionAndProofAggregationMigrationBlock: ULong,
    numberOfOperators: Int,
    l1ContractVersion: LinethRollupContractVersion,
  ): SafeFuture<ContactsDeploymentResult> {
    return deployLinethRollup(numberOfOperators, l1ContractVersion)
      .thenCombine(deployL2MessageService()) { linethRollupDeploymentResult, l2MessageServiceDeploymentResult ->
        ContactsDeploymentResult(
          linethRollup = linethRollupDeploymentResult,
          l2MessageService = l2MessageServiceDeploymentResult,
        )
      }
  }

  override fun connectToLinethRollupContract(
    contractAddress: String,
    transactionManager: AsyncFriendlyTransactionManager,
    gasProvider: ContractEIP1559GasProvider,
    smartContractErrors: SmartContractErrors?,
  ): LinethRollupSmartContractClient {
    return Web3JLinethRollupSmartContractClient.load(
      contractAddress = contractAddress,
      web3j = Web3jClientManager.l1Client,
      transactionManager = transactionManager,
      contractGasProvider = gasProvider,
      smartContractErrors = smartContractErrors ?: linethRollupContractErrors,
      ethLogsSearcher = EthLogsSearcherImpl(vertx = vertx, ethApiClient = EthApiClientManager.l1Client),
    )
  }
}

fun main() {
  data class SmartContractErrors(val smartContractErrors: Map<String, String>)

  val linethRollupContractErrors = findPathTo("config")!!
    .resolve("common/smart-contract-errors.toml")
    .let { filePath ->
      ConfigLoaderBuilder.default()
        .addFileSource(filePath.toAbsolutePath().toString())
        .build()
        .loadConfigOrThrow<SmartContractErrors>()
    }
  println(linethRollupContractErrors)
}
