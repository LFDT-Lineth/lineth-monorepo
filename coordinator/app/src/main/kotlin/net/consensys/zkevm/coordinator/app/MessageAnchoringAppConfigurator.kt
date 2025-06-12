package net.consensys.zkevm.coordinator.app

import io.vertx.core.Vertx
import linea.anchoring.MessageAnchoringApp
import linea.contract.l2.Web3JL2MessageServiceSmartContractClient
import linea.coordinator.config.v2.CoordinatorConfig
import linea.coordinator.config.v2.isDisabled
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import net.consensys.zkevm.LongRunningService
import org.apache.logging.log4j.LogManager

object MessageAnchoringAppConfigurator {
  fun create(
    vertx: Vertx,
    configs: CoordinatorConfig,
  ): LongRunningService {
    if (configs.messageAnchoring.isDisabled()) {
      LogManager.getLogger(MessageAnchoringApp::class.java).warn("Message anchoring is disabled")
      return DisabledLongRunningService
    }
    configs.messageAnchoring!!

    val l1Web3jClient = createWeb3jHttpClient(
      rpcUrl = configs.messageAnchoring.l1Endpoint.toString(),
      log = LogManager.getLogger("clients.l1.eth.message-anchoring"),
    )
    val l2Web3jClient = createWeb3jHttpClient(
      rpcUrl = configs.messageAnchoring.l2Endpoint.toString(),
      log = LogManager.getLogger("clients.l2.eth.message-anchoring"),
    )
    val l2TransactionManager = createTransactionManager(
      vertx = vertx,
      signerConfig = configs.messageAnchoring.signer,
      client = l2Web3jClient,
    )
    val messageAnchoringApp = MessageAnchoringApp(
      vertx = vertx,
      config = MessageAnchoringApp.Config(
        l1RequestRetryConfig = configs.messageAnchoring.l1RequestRetries,
        l1PollingInterval = configs.messageAnchoring.l1EventScrapping.pollingInterval,
        l1SuccessBackoffDelay = configs.messageAnchoring.l1EventScrapping.ethLogsSearchSuccessBackoffDelay,
        l1ContractAddress = configs.protocol.l1.contractAddress,
        l1EventPollingTimeout = configs.messageAnchoring.l1EventScrapping.pollingTimeout,
        l1EventSearchBlockChunk = configs.messageAnchoring.l1EventScrapping.ethLogsSearchBlockChunkSize,
        l1HighestBlockTag = configs.messageAnchoring.l1HighestBlockTag,
        l2HighestBlockTag = configs.messageAnchoring.l2HighestBlockTag,
        anchoringTickInterval = configs.messageAnchoring.anchoringTickInterval,
        messageQueueCapacity = configs.messageAnchoring.messageQueueCapacity,
        maxMessagesToAnchorPerL2Transaction = configs.messageAnchoring.maxMessagesToAnchorPerL2Transaction,
      ),
      l1EthApiClient = createEthApiClient(
        web3jClient = l1Web3jClient,
        requestRetryConfig = null,
        vertx = vertx,
      ),
      l2MessageService = Web3JL2MessageServiceSmartContractClient.create(
        web3jClient = l2Web3jClient,
        contractAddress = configs.protocol.l2.contractAddress,
        gasLimit = configs.messageAnchoring.gas.gasLimit,
        maxFeePerGasCap = configs.messageAnchoring.gas.maxFeePerGasCap,
        feeHistoryBlockCount = configs.messageAnchoring.gas.feeHistoryBlockCount,
        feeHistoryRewardPercentile = configs.messageAnchoring.gas.feeHistoryRewardPercentile.toDouble(),
        transactionManager = l2TransactionManager,
        smartContractErrors = configs.smartContractErrors,
        smartContractDeploymentBlockNumber = configs.protocol.l2.contractDeploymentBlockNumber?.getNumber(),
      ),
    )
    return messageAnchoringApp
  }
}
