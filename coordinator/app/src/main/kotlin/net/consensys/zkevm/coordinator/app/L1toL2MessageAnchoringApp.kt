package net.consensys.zkevm.coordinator.app

import io.vertx.core.Vertx
import linea.web3j.SmartContractErrors
import net.consensys.linea.contract.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.EIP1559GasProvider
import net.consensys.linea.contract.L2MessageService
import net.consensys.linea.contract.l1.Web3JLineaRollupSmartContractClient
import net.consensys.zkevm.LongRunningService
import net.consensys.zkevm.coordinator.app.config.L1Config
import net.consensys.zkevm.coordinator.app.config.L2Config
import net.consensys.zkevm.coordinator.app.config.MessageAnchoringServiceConfig
import net.consensys.zkevm.coordinator.app.config.SignerConfig
import net.consensys.zkevm.ethereum.coordination.messageanchoring.L1EventQuerier
import net.consensys.zkevm.ethereum.coordination.messageanchoring.L1EventQuerierImpl
import net.consensys.zkevm.ethereum.coordination.messageanchoring.L2MessageAnchorer
import net.consensys.zkevm.ethereum.coordination.messageanchoring.L2MessageAnchorerImpl
import net.consensys.zkevm.ethereum.coordination.messageanchoring.L2Querier
import net.consensys.zkevm.ethereum.coordination.messageanchoring.L2QuerierImpl
import net.consensys.zkevm.ethereum.coordination.messageanchoring.MessageAnchoringService
import org.apache.logging.log4j.LogManager
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.toKotlinDuration

class L1toL2MessageAnchoringApp(
  vertx: Vertx,
  configs: Config,
  l1Web3jClient: Web3j,
  l2Web3jClient: Web3j,
  smartContractErrors: SmartContractErrors,
  l2MessageService: L2MessageService,
  l2TransactionManager: AsyncFriendlyTransactionManager
) : LongRunningService {
  private val log = LogManager.getLogger(this::class.java)

  private var l1GasProvider = EIP1559GasProvider(
    l1Web3jClient,
    EIP1559GasProvider.Config(
      configs.l1.gasLimit,
      configs.l1.maxFeePerGasCap,
      configs.l1.feeHistoryBlockCount.toUInt(),
      configs.l1.feeHistoryRewardPercentile
    )
  )

  data class Config(
    val l1: L1Config,
    val l2: L2Config,
    val l1Signer: SignerConfig,
    val l2Signer: SignerConfig,
    val messageAnchoringService: MessageAnchoringServiceConfig
  )

  private val messageAnchoringService: MessageAnchoringService = run {
    val l1TransactionManager = createTransactionManager(
      vertx,
      configs.l1Signer,
      l1Web3jClient
    )

    val l1EventQuerier: L1EventQuerier = L1EventQuerierImpl(
      vertx,
      L1EventQuerierImpl.Config(
        configs.l1.sendMessageEventPollingInterval.toKotlinDuration(),
        configs.l1.maxEventScrapingTime.toKotlinDuration(),
        configs.l1.earliestBlock,
        configs.l1.maxMessagesToCollect,
        configs.l1.zkEvmContractAddress,
        configs.l1.finalizedBlockTag,
        configs.l1.blockRangeLoopLimit
      ),
      l1Web3jClient
    )

    val l2Querier: L2Querier = L2QuerierImpl(
      l2Web3jClient,
      l2MessageService,
      L2QuerierImpl.Config(
        blocksToFinalizationL2 = configs.l2.blocksToFinalization,
        lastHashSearchWindow = configs.l2.lastHashSearchWindow,
        contractAddressToListen = l2MessageService.contractAddress
      ),
      vertx
    )

    val l2MessageAnchorer: L2MessageAnchorer = L2MessageAnchorerImpl(
      vertx,
      l2Web3jClient,
      l2MessageService,
      L2MessageAnchorerImpl.Config(
        configs.l2.anchoringReceiptPollingInterval.toKotlinDuration(),
        configs.l2.maxReceiptRetries,
        configs.l2.blocksToFinalization.toLong()
      )
    )

    val anchoringConfig = MessageAnchoringService.Config(
      configs.messageAnchoringService.pollingInterval.toKotlinDuration(),
      configs.messageAnchoringService.maxMessagesToAnchor
    )

    val lineaRollupSmartContractClient = Web3JLineaRollupSmartContractClient.load(
      contractAddress = configs.l1.zkEvmContractAddress,
      web3j = l1Web3jClient,
      transactionManager = l1TransactionManager,
      contractGasProvider = l1GasProvider,
      smartContractErrors = smartContractErrors
    )

    MessageAnchoringService(
      anchoringConfig,
      vertx,
      l1EventQuerier,
      l2MessageAnchorer,
      l2Querier,
      lineaRollupSmartContractClient = lineaRollupSmartContractClient,
      l2MessageService,
      l2TransactionManager
    )
  }

  override fun start(): SafeFuture<Unit> {
    return messageAnchoringService.start().thenPeek {
      log.info("L1toL2MessageAnchoringApp started")
    }
  }

  override fun stop(): SafeFuture<Unit> {
    return messageAnchoringService.stop().thenPeek {
      log.info("L1toL2MessageAnchoringApp stopped")
    }
  }
}
