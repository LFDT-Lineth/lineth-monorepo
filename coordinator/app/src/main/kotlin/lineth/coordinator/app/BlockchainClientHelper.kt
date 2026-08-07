package lineth.coordinator.app

import io.vertx.core.Vertx
import lineth.contract.l1.LineaSmartContractClient
import lineth.contract.l1.Web3JLineaValidiumSmartContractClient
import lineth.contract.l1.Web3JLinethRollupSmartContractClient
import lineth.coordinator.config.v2.L1SubmissionConfig
import lineth.coordinator.config.v2.SignerConfig
import lineth.ethapi.EthLogsSearcherImpl
import lineth.ethereum.gaspricing.BoundableFeeCalculator
import lineth.ethereum.gaspricing.FeesCalculator
import lineth.ethereum.gaspricing.FeesFetcher
import lineth.ethereum.gaspricing.WMAGasProvider
import lineth.web3j.ECKeypairSignerAdapter
import lineth.web3j.SmartContractErrors
import lineth.web3j.ethapi.createEthApiClient
import lineth.web3j.transactionmanager.AsyncFriendlyTransactionManager
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.service.TxSignServiceImpl
import org.web3j.tx.gas.ContractGasProvider

fun createTransactionManager(
  vertx: Vertx,
  signerConfig: SignerConfig,
  client: Web3j,
  signerFactory: SignerFactory = DefaultSignerFactory,
): AsyncFriendlyTransactionManager {
  val signer = signerFactory.create(vertx, signerConfig)
  val credentials = Credentials.create(ECKeypairSignerAdapter(signer))
  return AsyncFriendlyTransactionManager(client, TxSignServiceImpl(credentials), -1L)
}

/**
 * @param useEthEstimateGas `eth_estimateGas` may revert for multi-blob data-submission txs;
 * disable it if it's the case.
 */
fun createLineaContractClient(
  vertx: Vertx,
  dataAvailabilityType: L1SubmissionConfig.DataAvailability,
  contractAddress: String,
  transactionManager: AsyncFriendlyTransactionManager,
  contractGasProvider: ContractGasProvider,
  web3jClient: Web3j,
  smartContractErrors: SmartContractErrors,
  useEthEstimateGas: Boolean,
): LineaSmartContractClient {
  return when (dataAvailabilityType) {
    L1SubmissionConfig.DataAvailability.ROLLUP ->
      Web3JLinethRollupSmartContractClient.load(
        contractAddress = contractAddress,
        web3j = web3jClient,
        transactionManager = transactionManager,
        contractGasProvider = contractGasProvider,
        smartContractErrors = smartContractErrors,
        ethLogsSearcher = EthLogsSearcherImpl(vertx = vertx, ethApiClient = createEthApiClient(web3jClient)),
        useEthEstimateGas = useEthEstimateGas,
      )

    L1SubmissionConfig.DataAvailability.VALIDIUM ->
      Web3JLineaValidiumSmartContractClient.load(
        contractAddress = contractAddress,
        web3j = web3jClient,
        transactionManager = transactionManager,
        contractGasProvider = contractGasProvider,
        smartContractErrors = smartContractErrors,
        useEthEstimateGas = useEthEstimateGas,
      )
  }
}

/**
 * @param useEthEstimateGas `eth_estimateGas` may revert for multi-blob data-submission txs;
 * disable it if it's the case.
 */
fun createLineaContractClient(
  l1ChainId: ULong,
  contractAddress: String,
  smartContractErrors: SmartContractErrors,
  vertx: Vertx,
  l1Web3jClient: Web3j,
  feesFetcher: FeesFetcher,
  signerConfig: SignerConfig,
  gasConfig: L1SubmissionConfig.GasConfig,
  l1MinPriorityFeeCalculator: FeesCalculator,
  dataAvailabilityType: L1SubmissionConfig.DataAvailability,
  signerFactory: SignerFactory = DefaultSignerFactory,
  useEthEstimateGas: Boolean = false,
): LineaSmartContractClient {
  val l1PriorityFeeCalculator: FeesCalculator = BoundableFeeCalculator(
    BoundableFeeCalculator.Config(
      feeUpperBound = gasConfig.fallback.priorityFeePerGasUpperBound.toDouble(),
      feeLowerBound = gasConfig.fallback.priorityFeePerGasLowerBound.toDouble(),
      feeMargin = 0.0,
    ),
    l1MinPriorityFeeCalculator,
  )
  // The below gas provider will act as the primary gas provider if L1
  // dynamic gas pricing is disabled and will act as a fallback gas provider
  // if L1 dynamic gas pricing is enabled
  val primaryOrFallbackGasProvider = WMAGasProvider(
    chainId = l1ChainId.toLong(),
    feesFetcher = feesFetcher,
    priorityFeeCalculator = l1PriorityFeeCalculator,
    config = WMAGasProvider.Config(
      gasLimit = gasConfig.gasLimit,
      maxFeePerGasCap = gasConfig.maxFeePerGasCap,
      maxPriorityFeePerGasCap = gasConfig.maxPriorityFeePerGasCap,
      maxFeePerBlobGasCap = gasConfig.maxFeePerBlobGasCap,
    ),
  )
  val transactionManager = createTransactionManager(
    vertx = vertx,
    signerConfig = signerConfig,
    client = l1Web3jClient,
    signerFactory = signerFactory,
  )
  return createLineaContractClient(
    vertx = vertx,
    dataAvailabilityType = dataAvailabilityType,
    contractAddress = contractAddress,
    transactionManager = transactionManager,
    contractGasProvider = primaryOrFallbackGasProvider,
    web3jClient = l1Web3jClient,
    smartContractErrors = smartContractErrors,
    useEthEstimateGas = useEthEstimateGas,
  )
}
