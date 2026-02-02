package linea.ftx

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.contract.events.FinalizedStateUpdatedEvent
import linea.contract.events.ForcedTransactionAddedEvent
import linea.contrat.events.FactoryFinalizedStateUpdatedEvent
import linea.contrat.events.FactoryForcedTransactionAddedEvent
import linea.domain.BlockParameter
import linea.domain.EthLog
import linea.ethapi.FakeEthApiClient
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.log4j.configureLoggers
import linea.persistence.ftx.FakeForcedTransactionsDao
import linea.persistence.ftx.ForcedTransactionsDao
import net.consensys.zkevm.domain.ForcedTransactionRecord
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
class ForcedTransactionsAppTest {
  private val L1_CONTRACT_ADDRESS = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01"
  private lateinit var l1Client: FakeEthApiClient
  private lateinit var vertx: Vertx
  private lateinit var fakeFinalStateProvider: FakeLineaRollupSmartContractClientReadOnlyFinalizedStateProvider
  private lateinit var ftxClient: FakeForcedTransactionsClient
  private lateinit var fxtDao: ForcedTransactionsDao

  @BeforeEach
  fun setUp(vertx: Vertx) {
    this.vertx = vertx
    l1Client = FakeEthApiClient(
      initialLogsDb = emptySet(),
      topicsTranslation = mapOf(
        FinalizedStateUpdatedEvent.topic to "FinalizedStateUpdated",
        ForcedTransactionAddedEvent.topic to "ForcedTransactionAdded",
      ),
      log = LogManager.getLogger("FakeEthApiClient"),
    )
    configureLoggers(
      rootLevel = Level.INFO,
      "FakeEthApiClient" to Level.INFO,
      "linea.ethapi" to Level.DEBUG,
      "linea.ftx" to Level.TRACE,
    )

    this.fakeFinalStateProvider = FakeLineaRollupSmartContractClientReadOnlyFinalizedStateProvider()
    this.ftxClient = FakeForcedTransactionsClient(errorRatio = 0.5)
    this.fxtDao = FakeForcedTransactionsDao()
  }

  private fun createApp(
    l1PollingInterval: Duration = 100.milliseconds,
    l1EventSearchBlockChunk: UInt = 1000u,
    ftxSequencerSendingInterval: Duration = 100.milliseconds,
  ): ForcedTransactionsApp {
    val config = ForcedTransactionsApp.Config(
      l1PollingInterval = l1PollingInterval,
      l1ContractAddress = L1_CONTRACT_ADDRESS,
      l1HighestBlockTag = BlockParameter.Tag.FINALIZED,
      l1EventSearchBlockChunk = l1EventSearchBlockChunk,
      ftxSequencerSendingInterval = ftxSequencerSendingInterval,
    )

    return ForcedTransactionsApp(
      config = config,
      vertx = vertx,
      l1EthApiClient = l1Client,
      finalizedStateProvider = this.fakeFinalStateProvider,
      ftxClient = this.ftxClient,
      ftxDao = this.fxtDao,
    )
  }

  fun createFtxAddedEvent(
    l1BlockNumber: ULong,
    ftxNumber: ULong,
    l2DeadLine: ULong,
  ): EthLog {
    return FactoryForcedTransactionAddedEvent.createEthLog(
      l1BlockNumber = l1BlockNumber,
      contractAddress = L1_CONTRACT_ADDRESS,
      forcedTransactionNumber = ftxNumber,
      blockNumberDeadline = l2DeadLine,
    )
  }

  @Test
  fun `should send ftx to the sequencer in order`() {
    val finalizedStateEvent =
      FactoryFinalizedStateUpdatedEvent.createEthLog(
        blockNumber = 1_000UL,
        contractAddress = L1_CONTRACT_ADDRESS,
        l2FinalizedBlockNumber = 100UL,
        forcedTransactionNumber = 10UL,
      )
    val ftxAddedEvents = listOf(
      createFtxAddedEvent(
        l1BlockNumber = 900UL,
        ftxNumber = 9UL,
        l2DeadLine = 90UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 990UL,
        ftxNumber = 10UL,
        l2DeadLine = 100UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 1_100UL,
        ftxNumber = 11UL,
        l2DeadLine = 200UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 1_200UL,
        ftxNumber = 12UL,
        l2DeadLine = 220UL,
      ),
    )
    l1Client.setLogs(listOf(finalizedStateEvent) + ftxAddedEvents)

    val app = createApp(
      l1PollingInterval = 200.milliseconds,
      l1EventSearchBlockChunk = 10u,
    )
    this.l1Client.setFinalizedBlockTag(5_000UL)
    this.l1Client.setLatestBlockTag(10_000UL)
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 11UL,
      l2BlockNumber = 2_000UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    app.start().get()

    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        // ftx 12 is expected to be duplicated because sequencer does not have inclusion status
        // so coordinator shall resend it in case sequencer restarts
        assertThat(ftxClient.ftxReceivedIds).startsWith(11UL, 12UL, 12UL)
      }

    assertThat(fxtDao.list().get()).isEqualTo(
      listOf(
        ForcedTransactionRecord(
          ftxNumber = 11UL,
          inclusionResult = ForcedTransactionInclusionResult.Included,
          simulatedExecutionBlockNumber = this.ftxClient.ftxInclusionResults[11UL]!!.blockNumber,
          simulatedExecutionBlockTimestamp = this.ftxClient.ftxInclusionResults[11UL]!!.blockTimestamp,
        ),
      ),
    )
  }

  //
  // should send handle duplicated ftx with different submissions order
  // should be resilient to coordinator/sequencer restarts ensure submission order without gaps
}
