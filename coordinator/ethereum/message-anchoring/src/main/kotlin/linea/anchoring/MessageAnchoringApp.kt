package linea.anchoring

import io.vertx.core.Vertx
import linea.EthLogsSearcher
import linea.anchoring.clients.L1MessageSentEventsPoller
import linea.contract.events.MessageSentEvent
import linea.contract.l2.L2MessageServiceSmartContractClient
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.ethapi.EthApiClient
import linea.ethapi.EthLogsSearcherImpl
import net.consensys.zkevm.LongRunningService
import org.apache.logging.log4j.LogManager
import java.util.Deque
import java.util.concurrent.CompletableFuture
import java.util.concurrent.LinkedBlockingDeque
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class MessageAnchoringApp(
  private val vertx: Vertx,
  private val config: Config,
  l1EthApiClient: EthApiClient,
  private val l2MessageService: L2MessageServiceSmartContractClient,
) : LongRunningService {
  data class Config(
    val l1RequestRetryConfig: RetryConfig,
    val l1PollingInterval: Duration = 12.seconds,
    val l1SuccessBackoffDelay: Duration = 1.milliseconds, // is configurable mostly for testing purposes
    val l1ContractAddress: String,
    val l1HighestBlockTag: BlockParameter,
    val l2HighestBlockTag: BlockParameter,
    val anchoringTickInterval: Duration,
    val l1EventPollingTimeout: Duration = 5.seconds,
    val l1EventSearchBlockChunk: UInt = 1000u,
    val messageQueueCapacity: UInt = 10_000u,
    val maxMessagesToAnchorPerL2Transaction: UInt = 100u,
  )
  private val log = LogManager.getLogger(MessageAnchoringApp::class.java)

  private val l1EthLogsSearcher: EthLogsSearcher =
    EthLogsSearcherImpl(
      vertx = vertx,
      ethApiClient = l1EthApiClient,
      config = EthLogsSearcherImpl.Config(loopSuccessBackoffDelay = config.l1SuccessBackoffDelay),
    )

  private val eventsQueue: Deque<MessageSentEvent> = LinkedBlockingDeque()
  internal val eventsQueueSize: Int
    get() = eventsQueue.size

  private val l1EventsPoller = run {
    L1MessageSentEventsPoller(
      vertx = vertx,
      pollingInterval = config.l1PollingInterval,
      l1SmartContractAddress = config.l1ContractAddress,
      l1EventsSearcher = l1EthLogsSearcher,
      eventsQueue = eventsQueue,
      eventsQueueMaxCapacity = config.messageQueueCapacity.toInt(),
      l2MessageService = l2MessageService,
      l1MessagesSentFetchLimit = config.maxMessagesToAnchorPerL2Transaction * 2u,
      l1MessagesSentFetchTimeout = config.l1EventPollingTimeout,
      l1BlockSearchChuck = config.l1EventSearchBlockChunk,
      l1HighestBlock = config.l1HighestBlockTag,
      l2HighestBlock = config.l2HighestBlockTag,
    )
  }
  private val messageAnchoringService =
    MessageAnchoringService(
      vertx = vertx,
      l1ContractAddress = config.l1ContractAddress,
      l1EthLogsClient = l1EthApiClient,
      l2MessageService = l2MessageService,
      eventsQueue = eventsQueue,
      maxMessagesToAnchorPerL2Transaction = config.maxMessagesToAnchorPerL2Transaction,
      l2HighestBlockTag = config.l2HighestBlockTag,
      anchoringTickInterval = config.anchoringTickInterval,
    )

  override fun start(): CompletableFuture<Unit> {
    log.info("starting MessageAnchoringApp with config={}", config)
    return l1EventsPoller.start()
      .thenCompose { messageAnchoringService.start() }
  }

  override fun stop(): CompletableFuture<Unit> {
    return CompletableFuture.allOf(
      l1EventsPoller.stop(),
      messageAnchoringService.stop(),
    ).thenApply {
      Unit
    }
  }
}
