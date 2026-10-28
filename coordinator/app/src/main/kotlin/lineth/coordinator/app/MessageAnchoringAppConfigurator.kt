package lineth.coordinator.app

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.contract.l2.Web3JL2MessageServiceSmartContractClient
import linea.ethapi.EthLogsSearcherImpl
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import lineth.anchoring.MessageAnchoringApp
import lineth.coordinator.config.v2.CoordinatorConfig
import lineth.coordinator.config.v2.isDisabled
import org.apache.logging.log4j.LogManager

object MessageAnchoringAppConfigurator {
  fun create(
    vertx: Vertx,
    configs: CoordinatorConfig,
    signerFactory: SignerFactory = DefaultSignerFactory,
  ): LongRunningService {
    if (configs.messageAnchoring.isDisabled()) {
      LogManager.getLogger(MessageAnchoringApp::class.java).warn("Message anchoring is disabled")
      return DisabledLongRunningService
    }
    val messageAnchoring = configs.messageAnchoring!!

    val l1EthApiClient =
      createEthApiClient(
        rpcUrl = messageAnchoring.l1Endpoint.toString(),
        log = LogManager.getLogger("clients.l1.eth.message-anchoring"),
        requestRetryConfig = messageAnchoring.l1RequestRetries,
        vertx = vertx,
      )
    val l2Web3jClient =
      createWeb3jHttpClient(
        rpcUrl = messageAnchoring.l2Endpoint.toString(),
        log = LogManager.getLogger("clients.l2.eth.message-anchoring"),
      )
    val l2EthApiClient =
      createEthApiClient(
        rpcUrl = messageAnchoring.l2Endpoint.toString(),
        log = LogManager.getLogger("clients.l2.eth.message-anchoring"),
      )
    val l2TransactionManager =
      createTransactionManager(
        vertx = vertx,
        signerConfig = messageAnchoring.signer,
        client = l2Web3jClient,
        signerFactory = signerFactory,
      )
    val messageAnchoringApp =
      MessageAnchoringApp(
        vertx = vertx,
        config =
        MessageAnchoringApp.Config(
          l1PollingInterval = messageAnchoring.l1EventScrapping.pollingInterval,
          l1SuccessBackoffDelay = messageAnchoring.l1EventScrapping.ethLogsSearchSuccessBackoffDelay,
          l1ContractAddress = configs.protocol.l1.contractAddress,
          l1EventPollingTimeout = messageAnchoring.l1EventScrapping.pollingTimeout,
          l1EventSearchBlockChunk = messageAnchoring.l1EventScrapping.ethLogsSearchBlockChunkSize,
          l1EventSearchMaxBlockRange = messageAnchoring.l1EventScrapping.ethLogsSearchMaxBlockRange,
          l1HighestBlockTag = messageAnchoring.l1HighestBlockTag,
          l2HighestBlockTag = messageAnchoring.l2HighestBlockTag,
          anchoringTickInterval = messageAnchoring.anchoringTickInterval,
          messageQueueCapacity = messageAnchoring.messageQueueCapacity,
          maxMessagesToAnchorPerL2Transaction = messageAnchoring.maxMessagesToAnchorPerL2Transaction,
        ),
        l1EthApiClient = l1EthApiClient,
        l2MessageService =
        Web3JL2MessageServiceSmartContractClient.create(
          web3jClient = l2Web3jClient,
          ethApiClient = l2EthApiClient,
          ethLogsSearcher = EthLogsSearcherImpl(vertx = vertx, ethApiClient = l2EthApiClient),
          contractAddress = configs.protocol.l2.contractAddress,
          gasLimit = messageAnchoring.gas.gasLimit,
          maxFeePerGasCap = messageAnchoring.gas.maxFeePerGasCap,
          feeHistoryBlockCount = messageAnchoring.gas.feeHistoryBlockCount,
          feeHistoryRewardPercentile = messageAnchoring.gas.feeHistoryRewardPercentile.toDouble(),
          transactionManager = l2TransactionManager,
          smartContractErrors = configs.smartContractErrors,
          smartContractDeploymentBlockNumber = configs.protocol.l2.contractDeploymentBlockNumber?.number,
        ),
      )
    return messageAnchoringApp
  }
}
