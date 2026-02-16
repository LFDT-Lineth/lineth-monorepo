package linea.ftx

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.contract.events.FinalizedStateUpdatedEvent
import linea.contract.events.ForcedTransactionAddedEvent
import linea.contract.l1.FakeLineaRollupSmartContractClient
import linea.contract.l1.LineaRollupContractVersion
import linea.contract.l1.LineaRollupFinalizedState
import linea.contrat.events.FactoryForcedTransactionAddedEvent
import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.EthLog
import linea.ethapi.EthApiBlockClient
import linea.ethapi.FakeEthApiClient
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.log4j.configureLoggers
import linea.persistence.ftx.FakeForcedTransactionsDao
import linea.persistence.ftx.ForcedTransactionsDao
import net.consensys.FakeFixedClock
import net.consensys.zkevm.domain.ForcedTransactionRecord
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit
import kotlin.concurrent.atomics.AtomicBoolean
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.days
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
@OptIn(ExperimentalAtomicApi::class)
class ForcedTransactionsAppTest {
  private val L1_CONTRACT_ADDRESS = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01"
  private lateinit var l1Client: FakeEthApiClient
  private lateinit var l2Client: FakeEthApiClient
  private lateinit var vertx: Vertx
  private lateinit var ftxClient: FakeForcedTransactionsClient
  private lateinit var fxtDao: ForcedTransactionsDao
  private lateinit var fakeContractClient: FakeLineaRollupSmartContractClient
  private val fakeClock = FakeFixedClock(Instant.parse("2025-01-01T00:00:00Z"))

  @BeforeEach
  fun setUp(vertx: Vertx) {
    configureLoggers(
      rootLevel = Level.INFO,
      "l1.FakeEthApiClient" to Level.INFO,
      "l2.FakeEthApiClient" to Level.INFO,
      "linea.ethapi" to Level.INFO,
      "linea.ftx" to Level.INFO,
      "linea.ftx.conflation" to Level.TRACE,
    )

    this.vertx = vertx
    this.l1Client = FakeEthApiClient(
      genesisTimestamp = fakeClock.now() - 11.seconds,
      blockTime = 12.seconds,
      topicsTranslation = mapOf(
        FinalizedStateUpdatedEvent.topic to "FinalizedStateUpdated",
        ForcedTransactionAddedEvent.topic to "ForcedTransactionAdded",
      ),
      log = LogManager.getLogger("l1.FakeEthApiClient"),
    )
    this.l2Client = FakeEthApiClient(
      log = LogManager.getLogger("l2.FakeEthApiClient"),
    )
    this.fakeContractClient = FakeLineaRollupSmartContractClient(
      contractVersion = LineaRollupContractVersion.V8,
    )
    this.fxtDao = FakeForcedTransactionsDao()
  }

  private fun createApp(
    l1PollingInterval: Duration = 10.milliseconds,
    l1EventSearchBlockChunk: UInt = 1000u,
    ftxSequencerSendingInterval: Duration = 100.milliseconds,
    ftxProcessingDelay: Duration = Duration.ZERO,
    fakeForcedTransactionsClientErrorRatio: Double = 0.5,
  ): ForcedTransactionsAppImpl {
    val config = ForcedTransactionsApp.Config(
      l1PollingInterval = l1PollingInterval,
      l1ContractAddress = L1_CONTRACT_ADDRESS,
      l1HighestBlockTag = BlockParameter.Tag.LATEST,
      l1EventSearchBlockChunk = l1EventSearchBlockChunk,
      ftxSequencerSendingInterval = ftxSequencerSendingInterval,
      ftxProcessingDelay = ftxProcessingDelay,
    )

    this.ftxClient = FakeForcedTransactionsClient(errorRatio = fakeForcedTransactionsClientErrorRatio)
    return ForcedTransactionsAppImpl(
      config = config,
      vertx = this.vertx,
      l1EthApiClient = this.l1Client,
      finalizedStateProvider = this.fakeContractClient,
      contractVersionProvider = this.fakeContractClient,
      ftxClient = this.ftxClient,
      ftxDao = this.fxtDao,
      l2EthApiClient = this.l2Client,
      clock = fakeClock,
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
  fun `should send ftx to the sequencer in order and handle conflation correctly`() {
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
    this.l1Client.setLogs(ftxAddedEvents)
    this.fakeContractClient.finalizedStateProvider.l1FinalizedState = LineaRollupFinalizedState(
      blockNumber = 100UL,
      blockTimestamp = Clock.System.now(),
      messageNumber = 0UL,
      forcedTransactionNumber = 10UL,
    )

    val app = createApp(
      l1PollingInterval = 10.milliseconds,
      l1EventSearchBlockChunk = 100u,
      fakeForcedTransactionsClientErrorRatio = 0.0,
    )
    this.l1Client.setFinalizedBlockTag(5_000UL)
    this.l1Client.setLatestBlockTag(10_000UL)
    this.l2Client.setLatestBlockTag(2_000UL)
    val ftxL2ProcessedBlockNumber = 2_010UL
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 11UL,
      l2BlockNumber = ftxL2ProcessedBlockNumber,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    this.fakeClock.setTimeTo(this.l1Client.blockTimestamp(BlockParameter.Tag.LATEST) + 12.seconds)
    app.start().get()
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

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
          proofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
          proofIndex = null,
        ),
      ),
    )
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber())
      .isEqualTo(ftxL2ProcessedBlockNumber)

    // simulate that sequencer processed ftx 12
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 12UL,
      l2BlockNumber = 2_020UL,
      inclusionResult = ForcedTransactionInclusionResult.BadNonce,
    )

    // there are no more FTX to process, it should release the safe block number
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
      }
  }

  @Test
  fun `should let conflation resume when there are no ftx to process`() {
    // Scenario: FTX 10 has been finalized, no new FTXs to process
    val ftx10AddedEvent = createFtxAddedEvent(
      l1BlockNumber = 990UL,
      ftxNumber = 10UL,
      l2DeadLine = 100UL,
    )
    l1Client.setLogs(listOf(ftx10AddedEvent))

    // Configure the fake contract client to return the correct finalized state
    fakeContractClient.finalizedStateProvider.l1FinalizedState = LineaRollupFinalizedState(
      blockNumber = 100UL,
      blockTimestamp = Clock.System.now(),
      messageNumber = 0UL,
      forcedTransactionNumber = 10UL,
    )

    val app = createApp(
      l1PollingInterval = 100.milliseconds,
      ftxSequencerSendingInterval = 100.milliseconds,
    )
    this.l1Client.setFinalizedBlockTag(5_000UL)
    this.l1Client.setLatestBlockTag(10_000UL)
    this.l2Client.setLatestBlockTag(2_000UL)
    this.fakeClock.setTimeTo(this.l1Client.blockTimestamp(BlockParameter.Tag.LATEST) + 12.seconds)

    app.start()
      .also {
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
      }
      .get()

    // Wait a bit to ensure the app has time to process events
    await()
      .atMost(2.seconds.toJavaDuration())
      .untilAsserted {
        // Safe block number should remain at 0 when there are no forced transactions
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
      }

    // Verify no ftx were sent to the sequencer
    assertThat(ftxClient.ftxReceivedIds).isEmpty()
    // Verify no ftx records were created
    assertThat(fxtDao.list().get()).isEmpty()
  }

  @Test
  fun `on restart shall wait to reach the tip of the chain before releasing the conflation`() {
    // Scenario: On restart, there are FTX on L1 and may take long time to fetch..
    // The coordinator should keep the lock at 0 until it catches up with head or until it fetches the first one and
    // sends to the sequencer and gets inclusion status, whichever comes first.
    //
    // In this test we simulate the scenario where the coordinator needs to catch up with L1 history
    // and confirm there are no pending FTXs before releasing the lock.
    // Only after catching up should it release the lock
    // Configure the fake contract client to return the correct finalized state
    fakeContractClient.finalizedStateProvider.l1FinalizedState = LineaRollupFinalizedState(
      blockNumber = 100UL,
      blockTimestamp = Clock.System.now(),
      messageNumber = 0UL,
      forcedTransactionNumber = 10UL,
    )

    val ftxAddedEvents = listOf(
      createFtxAddedEvent(
        l1BlockNumber = 1_000UL,
        ftxNumber = 10UL,
        l2DeadLine = 100UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 11_110UL,
        ftxNumber = 11UL,
        l2DeadLine = 1000UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 12_120UL,
        ftxNumber = 12UL,
        l2DeadLine = 2000UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 13_130UL,
        ftxNumber = 13UL,
        l2DeadLine = 3000UL,
      ),
    )
    l1Client.setLogs(ftxAddedEvents)

    val app = createApp(
      l1PollingInterval = 10.milliseconds,
      l1EventSearchBlockChunk = 100u, // Small chunks to simulate slow catch-up
      ftxSequencerSendingInterval = 5.milliseconds, // to empty the queue faster once it starts sending
      fakeForcedTransactionsClientErrorRatio = 0.0,
    )
    this.l1Client.setFinalizedBlockTag(20_000UL)
    this.l2Client.setLatestBlockTag(900UL)
    this.fakeClock.setTimeTo(this.l1Client.blockTimestamp(BlockParameter.Tag.LATEST) + 12.seconds)

    val safeBlockTracker = SafeBlockTracker(app)
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
    app.start().get()
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(safeBlockTracker.stateTransitions.last()).isEqualTo(900UL)
      }

    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 11UL,
      l2BlockNumber = 1011UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(safeBlockTracker.stateTransitions.last()).isEqualTo(1011UL)
      }
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 12UL,
      l2BlockNumber = 1012UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 13UL,
      l2BlockNumber = 1013UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    await()
      .atMost(1.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(safeBlockTracker.stateTransitions.last()).isNull()
      }
    safeBlockTracker.stop()
    assertThat(safeBlockTracker.stateTransitions).startsWith(
      0UL, // start blocked
      900UL, // blocks conflation before sending 1st FTXs to sequencer
      1011UL, // blocks conflation at FTX processing block Number
    )
    app.stop().get()
  }

  @Test
  fun `should be resilient to coordinator and sequencer restarts ensure submission order without gaps`() {
    // scenario: sequencer restarts and loses the in-memory state of already processed ftx
    // Coordinator should resend FTXs until sequencer confirms processing
    // Configure the fake contract client to return the correct finalized state
    fakeContractClient.finalizedStateProvider.l1FinalizedState = LineaRollupFinalizedState(
      blockNumber = 100UL,
      blockTimestamp = Clock.System.now(),
      messageNumber = 0UL,
      forcedTransactionNumber = 9UL,
    )

    // Setup: FTX 10 was already processed before coordinator restart
    val ftx10Record = ForcedTransactionRecord(
      ftxNumber = 10UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
      simulatedExecutionBlockNumber = 2_010UL,
      simulatedExecutionBlockTimestamp = Clock.System.now(),
      proofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
      proofIndex = null,
    )
    fxtDao.save(ftx10Record).get()

    val ftxAddedEvents = listOf(
      createFtxAddedEvent(
        l1BlockNumber = 900UL,
        ftxNumber = 9UL,
        l2DeadLine = 90UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 1_000UL,
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
      createFtxAddedEvent(
        l1BlockNumber = 1_300UL,
        ftxNumber = 13UL,
        l2DeadLine = 240UL,
      ),
    )
    l1Client.setLogs(ftxAddedEvents)

    val app = createApp(
      l1PollingInterval = 100.milliseconds,
      ftxSequencerSendingInterval = 100.milliseconds,
      fakeForcedTransactionsClientErrorRatio = 0.0,
    )
    this.l1Client.setFinalizedBlockTag(5_000UL)
    this.l1Client.setLatestBlockTag(10_000UL)
    this.l2Client.setLatestBlockTag(2_000UL)
    this.fakeClock.setTimeTo(this.l1Client.blockTimestamp(BlockParameter.Tag.LATEST) + 12.seconds)

    // Sequencer initially doesn't have status for FTX 11 (simulating restart)
    // but will return status after the FTX is sent
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 11UL,
      l2BlockNumber = 2_020UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 12UL,
      l2BlockNumber = 2_030UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )

    app.start().get()

    // Should resume from the last processed FTX (10)
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(2_010UL)

    // Wait for FTX 11 and 12 to be sent and processed
    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        // FTX 11 and 12 should be sent (potentially multiple times until status is confirmed)
        assertThat(ftxClient.ftxReceivedIds).contains(11UL, 12UL)
        // Both should be in the database
        val records = fxtDao.list().get()
        assertThat(records.map { it.ftxNumber }).contains(11UL, 12UL)
      }

    // FTX 13 should be sent multiple times because sequencer doesn't have inclusion status
    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(ftxClient.ftxReceivedIds.count { it == 13UL }).isGreaterThan(1)
      }

    // Verify processing order: no gaps, sequential processing
    val allRecords = fxtDao.list().get()
    assertThat(allRecords.map { it.ftxNumber }).containsExactly(10UL, 11UL, 12UL)

    // Safe block number should be updated to the highest processed FTX block number
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(2_030UL)
  }

  @Test
  fun `should re-lock safe block number after contract upgrade from V7 to V8 as soon as 1st is sent to sequencer`() {
    // Scenario: Contract starts at V6/V7 and later on gets upgraded to V8.
    // forcedTransactionsUnsupportedYetByL1Contract() sets safeBlockNumber=null.
    // After awaitVersion() returns, the startup scan resumes.
    // The safe block number should be re-locked at 0 to prevent unrestricted conflation
    // while forced transactions are being discovered and sent to the sequencer.
    //
    // Bug: lockSafeBlockNumberBeforeSendingToSequencer early-returns when !startUpScanFinished,
    // so if safeBlockNumber is null (instead of 0), conflation remains unrestricted during scan.
    this.fakeContractClient.contractVersion = LineaRollupContractVersion.V7

    // FTX events already on L1 (submitted right after V8 upgrade)
    val ftxAddedEvents = listOf(
      createFtxAddedEvent(
        l1BlockNumber = 5_000UL,
        ftxNumber = 1UL,
        l2DeadLine = 1000UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 15_000UL,
        ftxNumber = 2UL,
        l2DeadLine = 2000UL,
      ),
    )
    l1Client.setLogs(ftxAddedEvents)

    // No prior finalized FTX (fresh V8 upgrade)
    fakeContractClient.finalizedStateProvider.l1FinalizedState = LineaRollupFinalizedState(
      blockNumber = 100UL,
      blockTimestamp = Clock.System.now(),
      messageNumber = 0UL,
      forcedTransactionNumber = 0UL,
    )

    // Large block range with small chunks to ensure scanning takes time
    this.l1Client.setFinalizedBlockTag(20_000UL)
    this.l1Client.setLatestBlockTag(20_000UL)
    this.l2Client.setLatestBlockTag(500UL)
    this.fakeClock.setTimeTo(this.l1Client.blockTimestamp(BlockParameter.Tag.LATEST) + 12.seconds)

    val app = createApp(
      l1PollingInterval = 10.milliseconds,
      l1EventSearchBlockChunk = 100u, // Small chunks to slow down catch-up
      ftxSequencerSendingInterval = 5.milliseconds,
      fakeForcedTransactionsClientErrorRatio = 0.0,
    )

    val safeBlockTracker = SafeBlockTracker(app)
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

    val startFuture = app.start()
    // Upgrade to V8 so awaitVersion completes
    this.fakeContractClient.contractVersion = LineaRollupContractVersion.V8
    startFuture.get(5, TimeUnit.SECONDS)

    // it should lock safe block number with l2 chain head
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)
      }

    // FTX should eventually be sent to sequencer and processed
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 1UL,
      l2BlockNumber = 510UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 2UL,
      l2BlockNumber = 520UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )

    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
        assertThat(safeBlockTracker.stateTransitions).startsWith(
          0UL, // initial lock
          null, // unlock while smart contract is not upgraded
          500UL, // lock on first ftx sent to sequencer
          510UL,
          520UL,
          null,
        )
      }

    safeBlockTracker.stop()
    app.stop().get()
  }

  @Test
  fun `should not hold the conflation while smart contract is not upgraded`() {
    this.fakeContractClient.contractVersion = LineaRollupContractVersion.V7
    val app = createApp(
      l1PollingInterval = 10.milliseconds,
      ftxSequencerSendingInterval = 100.milliseconds,
    )
    val startFuture = app.start()
    val safeBlockTracker = SafeBlockTracker(app)
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()

    // simulate upgrade, app start should complete after the upgrade
    // Wait for the app to start
    this.fakeContractClient.contractVersion = LineaRollupContractVersion.V8
    startFuture.get(3, TimeUnit.SECONDS)
    // it should still not hold the conflation
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()

    // simulate an edge case: FTX is sent to L1 after contract upgrade to V8 and before next Finalization
    // so app starts without FinalizedStateUpdated event
    val ftxAddedEvent = createFtxAddedEvent(
      l1BlockNumber = 1_000UL,
      ftxNumber = 1UL,
      l2DeadLine = 100UL,
    )
    l1Client.setLogs(listOf(ftxAddedEvent))
    l1Client.setFinalizedBlockTag(1_002UL)
    l2Client.setLatestBlockTag(10UL)
    this.fakeClock.setTimeTo(this.l1Client.blockTimestamp(BlockParameter.Tag.LATEST) + 12.seconds)

    await()
      .atMost(2.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(10UL)
        assertThat(ftxClient.ftxReceivedIds).contains(1UL)
      }

    l2Client.setLatestBlockTag(200UL)
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 1UL,
      l2BlockNumber = 100UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )

    await()
      .atMost(2.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
      }

    safeBlockTracker.stop()
    assertThat(safeBlockTracker.stateTransitions).containsExactly(
      null, // initial state, no lock
      10UL, // locked at L2 block number when processing FTX 1
      null, // released after processing FTX 1
    )
  }

  @Test
  fun `should process ftx only after processDelay has elapsed`() {
    // Setup: FTX 1 and 2 will be added to L1
    // Using low block numbers so timestamps are close to genesisTimestamp (recent)
    val ftxAddedEvents = listOf(
      createFtxAddedEvent(
        l1BlockNumber = 100UL,
        ftxNumber = 1UL,
        l2DeadLine = 200UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 200UL, // 100*12s = 20min after ftx1
        ftxNumber = 2UL,
        l2DeadLine = 300UL,
      ),
    )

    this.l1Client.setLogs(ftxAddedEvents)

    // Configure the fake contract client
    fakeContractClient.finalizedStateProvider.l1FinalizedState = LineaRollupFinalizedState(
      blockNumber = 100UL,
      blockTimestamp = Clock.System.now(),
      messageNumber = 0UL,
      forcedTransactionNumber = 0UL,
    )

    // Configure block tags
    this.l1Client.setFinalizedBlockTag(500UL)
    this.l1Client.setLatestBlockTag(1000UL)
    l2Client.setLatestBlockTag(150UL)

    val processingDelay = 3.days
    // Create app with processing delay using the special L1 client
    val app = createApp(
      ftxProcessingDelay = processingDelay,
      fakeForcedTransactionsClientErrorRatio = 0.0,
    )
    app.start().get()
    await()
      .atMost(2.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(app.ftxQueue).hasSize(2)
        assertThat(ftxClient.ftxReceivedIds).isEmpty()
      }

    // move the clock forward between the two FTXs to simulate the dela
    // and ensure only 1st one is eligible for processing
    val ftx1Time = this.l1Client.blockTimestamp(100UL.toBlockParameter())
    val ftx2Time = this.l1Client.blockTimestamp(200UL.toBlockParameter())
    // sanity check of test setup: validate time is set properly
    assertThat(ftx2Time - ftx1Time).isGreaterThan(10.minutes)
    fakeClock.setTimeTo(ftx1Time.plus(processingDelay).plus(1.seconds))

    await()
      .atMost(2.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(ftxClient.ftxReceivedIds).startsWith(1UL)
      }
    assertThat(ftxClient.ftxReceivedIds).doesNotContain(2UL)

    app.stop().get()
  }

  private fun EthApiBlockClient.blockTimestamp(blockParameter: BlockParameter): Instant =
    this.ethGetBlockByNumberTxHashes(blockParameter)
      .get().timestamp.let { Instant.fromEpochSeconds(it.toLong()) }

  class SafeBlockTracker(
    private val app: ForcedTransactionsApp,
    private val pollInterval: Duration = Duration.ZERO,
    private val startRightAway: Boolean = true,
  ) {
    val stateTransitions = CopyOnWriteArrayList<ULong?>()
    val keepRunning = AtomicBoolean(startRightAway)
    private var thread: Thread = Thread(this::run)
      .also {
        it.isDaemon = true
        if (startRightAway) {
          it.start()
        }
      }

    fun stop() {
      keepRunning.store(false)
      thread.interrupt()
    }

    private fun run() {
      var lastState: ULong? = ULong.MAX_VALUE
      runCatching {
        while (keepRunning.load()) {
          val safeBlockNumber = app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()
          if (lastState != safeBlockNumber) {
            lastState = safeBlockNumber
            stateTransitions.add(safeBlockNumber)
          }
          if (pollInterval >= 1.milliseconds) {
            Thread.sleep(pollInterval.inWholeMilliseconds)
          }
        }
      }
    }
  }
}
