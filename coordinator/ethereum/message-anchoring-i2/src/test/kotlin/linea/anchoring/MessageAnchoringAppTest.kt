package linea.anchoring

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.anchoring.fakes.FakeL2MessageService
import linea.contract.events.L1RollingHashUpdatedEvent
import linea.contract.events.L2RollingHashUpdatedEvent
import linea.contract.events.MessageSentEvent
import linea.contrat.events.L1MessageSentV1EthLogs
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.ethapi.FakeEthApiClient
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.kotlin.toHexStringUInt256
import linea.log4j.configureLoggers
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class MessageAnchoringAppTest {

  private val L1_CONTRACT_ADDRESS = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01"
  private val L2_CONTRACT_ADDRESS = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa02"
  private lateinit var l1Client: FakeEthApiClient
  private lateinit var l2MessageService: FakeL2MessageService
  private lateinit var vertx: Vertx

  @BeforeEach
  fun setUp(vertx: Vertx) {
    this.vertx = vertx
    l2MessageService = FakeL2MessageService(contractAddress = L2_CONTRACT_ADDRESS)
    l1Client = FakeEthApiClient(
      initialLogsDb = emptySet(),
      topicsTranslation = mapOf(
        L1RollingHashUpdatedEvent.topic to "L1RollingHashUpdatedEvent",
        MessageSentEvent.topic to "MessageSentEvent",
        L2RollingHashUpdatedEvent.topic to "L2RollingHashUpdatedEvent"
      ),
      log = LogManager.getLogger("FakeEthApiClient")
    )

    configureLoggers(
      rootLevel = Level.INFO,
      "FakeEthApiClient" to Level.INFO
    )
  }

  private fun createApp(
    anchoringTickInterval: Duration = 100.milliseconds,
    l1PollingInterval: Duration = 100.milliseconds,
    l1EventSearchBlockChunk: UInt = 1000u,
    l1EventPollingTimeout: Duration = 2.seconds,
    l1SuccessBackoffDelay: Duration = 1.milliseconds,
    messageQueueCapacity: UInt = 100u,
    maxMessagesToAnchorPerL2Transaction: UInt = 10u
  ): MessageAnchoringApp {
    return MessageAnchoringApp(
      vertx = vertx,
      config = MessageAnchoringApp.Config(
        l1PollingInterval = l1PollingInterval,
        l1SuccessBackoffDelay = l1SuccessBackoffDelay,
        l1ContractAddress = L1_CONTRACT_ADDRESS,
        l2HighestBlockTag = BlockParameter.Tag.LATEST,
        anchoringTickInterval = anchoringTickInterval,
        l1RequestRetryConfig = RetryConfig.noRetries,
        l1EventPollingTimeout = l1EventPollingTimeout,
        l1EventSearchBlockChunk = l1EventSearchBlockChunk,
        messageQueueCapacity = messageQueueCapacity,
        maxMessagesToAnchorPerL2Transaction = maxMessagesToAnchorPerL2Transaction
      ),
      l1EthApiClient = l1Client,
      l2MessageService = l2MessageService
    )
  }

  private fun addLogsToFakeEthClient(
    logs: List<L1MessageSentV1EthLogs>
  ) {
    l1Client.setLogs(logs.map { listOf(it.l1RollingHashUpdated.log, it.messageSent.log) }.flatten())
  }

  @Test
  fun `should anchor messages from fresh deployment`() {
    val ethLogs = createL1MessageSentV1Logs(
      l1BlocksWithMessages = listOf(100UL, 200UL, 300UL, 400UL),
      numberOfMessagesPerBlock = 1
    )
    addLogsToFakeEthClient(ethLogs)

    val anchoringApp = createApp(anchoringTickInterval = 100.milliseconds, l1EventSearchBlockChunk = 100u)
    anchoringApp.start().get()
    l1Client.setFinalizedBlockTag(ethLogs.last().messageSent.log.blockNumber + 10UL)
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(l2MessageService.getLastAnchoredL1MessageNumber(block = BlockParameter.Tag.LATEST).get())
          .isEqualTo(ethLogs.last().messageSent.event.messageNumber)
        assertThat(l2MessageService.getLastAnchoredRollingHash())
          .isEqualTo(ethLogs.last().l1RollingHashUpdated.event.rollingHash)
      }

    anchoringApp.stop().get()
  }

  @Test
  fun `should anchor messages resuming from latest anchored message on L2`() {
    val ethLogs = createL1MessageSentV1Logs(
      l1BlocksWithMessages = listOf(100UL, 200UL, 300UL, 400UL),
      numberOfMessagesPerBlock = 1
    )
    l2MessageService.setLastAnchoredL1Message(
      l1MessageNumber = ethLogs.first().l1RollingHashUpdated.event.messageNumber,
      rollingHash = ethLogs.first().l1RollingHashUpdated.event.rollingHash
    )
    val anchoringApp = createApp(
      anchoringTickInterval = 100.milliseconds,
      l1EventSearchBlockChunk = 100u
    )
    anchoringApp.start().get()
    addLogsToFakeEthClient(ethLogs)
    l1Client.setFinalizedBlockTag(ethLogs.last().messageSent.log.blockNumber + 10UL)

    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(l2MessageService.getLastAnchoredL1MessageNumber(block = BlockParameter.Tag.LATEST).get())
          .isEqualTo(ethLogs.last().l1RollingHashUpdated.event.messageNumber)
        assertThat(l2MessageService.getLastAnchoredRollingHash())
          .isEqualTo(ethLogs.last().l1RollingHashUpdated.event.rollingHash)

        assertThat(l2MessageService.getAnchoredMessageHashes().map { it.encodeHex() })
          .isEqualTo(ethLogs.drop(1).map { it.messageSent.event.messageHash.encodeHex() })
      }

    anchoringApp.stop().get()
  }

  @Test
  fun `should anchor messages in chunks when too many messages are waiting on L1`() {
    val ethLogs = createL1MessageSentV1Logs(
      l1BlocksWithMessages = (1UL..100UL).map { it },
      numberOfMessagesPerBlock = 1
    )
    val anchoringApp = createApp(
      l1EventSearchBlockChunk = 1000u,
      maxMessagesToAnchorPerL2Transaction = 20u,
      anchoringTickInterval = 1.milliseconds
    )
    anchoringApp.start().get()

    addLogsToFakeEthClient(ethLogs)
    l1Client.setFinalizedBlockTag(ethLogs.last().messageSent.log.blockNumber + 10UL)

    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(l2MessageService.getLastAnchoredL1MessageNumber(block = BlockParameter.Tag.LATEST).get())
          .isEqualTo(ethLogs.last().l1RollingHashUpdated.event.messageNumber)
      }

    assertThat(l2MessageService.getAnchoredMessagesRollingHashes().size).isEqualTo(5)

    anchoringApp.stop().get()
  }

  @Test
  fun `should be resilient to low activity or downtimes and find messages on L1 spread apart and anchor them`() {
    // put Log every 1000 Blocks on L1
    // search chunk is 10 so it needs 100 queries to find 1 log on L1
    // slow timeout 1
    val ethLogs = createL1MessageSentV1Logs(
      l1BlocksWithMessages = (1UL..100_000UL).step(1000).map { it },
      numberOfMessagesPerBlock = 1
    )
    val anchoringApp = createApp(
      l1PollingInterval = 1.milliseconds,
      l1EventSearchBlockChunk = 10u,
      // for this scenario l1EventPollingTimeout < l1SuccessBackoffDelay
      // to simulate timeout after the 1st response and return, doing next request on next tick
      l1EventPollingTimeout = 1.milliseconds,
      l1SuccessBackoffDelay = 3.milliseconds,
      maxMessagesToAnchorPerL2Transaction = 50u,
      anchoringTickInterval = 20.milliseconds
    )

    addLogsToFakeEthClient(ethLogs)
    l1Client.setFinalizedBlockTag(ethLogs.last().messageSent.log.blockNumber + 10UL)

    anchoringApp.start().get()
    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(l2MessageService.getLastAnchoredL1MessageNumber(block = BlockParameter.Tag.LATEST).get())
          .isEqualTo(ethLogs.last().l1RollingHashUpdated.event.messageNumber)
      }

    assertThat(l2MessageService.getAnchoredMessageHashes().map { it.encodeHex() })
      .isEqualTo(ethLogs.map { it.messageSent.event.messageHash.encodeHex() })

    anchoringApp.stop().get()
  }

  @Test
  fun `should be resilient when queue gets full`() {
    // Worst case scenario: L1 block has more messages that the queue can handle,
    // so it needs to query this block multiple times
    val ethLogs = createL1MessageSentV1Logs(
      l1BlocksWithMessages = listOf(100UL, 200UL, 201UL),
      numberOfMessagesPerBlock = 100,
      startingMessageNumber = 1UL
    )

    val anchoringApp = createApp(
      l1PollingInterval = 1.milliseconds,
      l1EventSearchBlockChunk = 10u,
      l1EventPollingTimeout = 1.seconds,
      messageQueueCapacity = 20u,
      maxMessagesToAnchorPerL2Transaction = 50u,
      anchoringTickInterval = 10.milliseconds
    )

    addLogsToFakeEthClient(ethLogs)
    l1Client.setFinalizedBlockTag(ethLogs.last().messageSent.log.blockNumber + 10UL)

    anchoringApp.start().get()
    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(l2MessageService.getLastAnchoredL1MessageNumber(block = BlockParameter.Tag.LATEST).get())
          .isEqualTo(ethLogs.last().l1RollingHashUpdated.event.messageNumber)
      }

    assertThat(l2MessageService.getAnchoredMessageHashes())
      .isEqualTo(ethLogs.map { it.messageSent.event.messageHash })

    anchoringApp.stop().get()
  }

  private fun createL1MessageSentV1Logs(
    l1BlocksWithMessages: List<ULong>,
    numberOfMessagesPerBlock: Int,
    startingMessageNumber: ULong = 1UL
  ): List<L1MessageSentV1EthLogs> {
    val ethLogs = mutableListOf<L1MessageSentV1EthLogs>()
    var messageNumber = startingMessageNumber
    l1BlocksWithMessages.forEach { blockNumber ->
      repeat(numberOfMessagesPerBlock) {
        ethLogs.add(
          linea.contrat.events.createL1MessageSentV1Logs(
            blockNumber = blockNumber,
            contractAddress = L1_CONTRACT_ADDRESS,
            messageNumber = messageNumber,
            messageHash = messageNumber.toHexStringUInt256().decodeHex(),
            rollingHash = messageNumber.toHexStringUInt256().decodeHex()
          )
        )
        messageNumber++
      }
    }
    return ethLogs
  }
}
