package linea.ftx

import build.linea.clients.StateManagerAccountProofClient
import build.linea.clients.StateManagerClientV1
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.InvalidityProverClientV1
import linea.clients.TracesConflationVirtualBlockClientV1
import linea.contract.events.FactoryForcedTransactionAddedEvent
import linea.contract.events.FinalizedStateUpdatedEvent
import linea.contract.events.ForcedTransactionAddedEvent
import linea.contract.l1.FakeLineaRollupSmartContractClient
import linea.contract.l1.LineaRollupContractVersion
import linea.contract.l1.LineaRollupFinalizedState
import linea.domain.BlobCounters
import linea.domain.BlockCounters
import linea.domain.BlockInterval
import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.EthLog
import linea.ethapi.EthApiBlockClient
import linea.ethapi.FakeEthApiClient
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.ftx.conflation.ForcedTransactionConflationSafeBlockNumberProvider
import linea.ftx.conflation.SafeBlockNumberUpdateListener
import linea.log4j.configureLoggers
import linea.persistence.ForcedTransactionRecord
import linea.persistence.ForcedTransactionsDao
import linea.persistence.ftx.FakeForcedTransactionsDao
import net.consensys.FakeFixedClock
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCountersV5
import net.consensys.linea.traces.TracingModuleV5
import net.consensys.zkevm.coordinator.clients.FakeTracesConflationVirtualBlockClientV1
import net.consensys.zkevm.ethereum.coordination.DynamicBlockNumberSet
import net.consensys.zkevm.ethereum.coordination.aggregation.AggregationCalculatorFactory
import net.consensys.zkevm.ethereum.coordination.blob.FakeBlobCompressor
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorFactory
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.days
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class ForcedTransactionsAppTest {
  private val L1_CONTRACT_ADDRESS = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01"
  private lateinit var l1Client: FakeEthApiClient
  private lateinit var l2Client: FakeEthApiClient
  private lateinit var vertx: Vertx
  private lateinit var ftxClient: FakeForcedTransactionsClient
  private lateinit var fxtDao: ForcedTransactionsDao
  private lateinit var fakeContractClient: FakeLineaRollupSmartContractClient
  private val fakeClock = FakeFixedClock(Instant.parse("2025-01-01T00:00:00Z"))
  private lateinit var invalidityProofClient: InvalidityProverClientV1
  private lateinit var stateManagerClient: StateManagerClientV1
  private lateinit var accountProofClient: StateManagerAccountProofClient
  private lateinit var tracesClient: TracesConflationVirtualBlockClientV1

  @BeforeEach
  fun setUp(vertx: Vertx) {
    configureLoggers(
      rootLevel = Level.INFO,
      "l1.FakeEthApiClient" to Level.INFO,
      "l2.FakeEthApiClient" to Level.INFO,
      "linea.ethapi" to Level.INFO,
      "linea.ftx" to Level.INFO,
      "linea.ftx.conflation" to Level.INFO,
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
    this.tracesClient = FakeTracesConflationVirtualBlockClientV1()
    this.invalidityProofClient = FakeInvalidityProverClient()
    FakeStateManagerClient()
      .also {
        this.stateManagerClient = it
        this.accountProofClient = it
      }
  }

  private fun createApp(
    l1PollingInterval: Duration = 10.milliseconds,
    l1EventSearchBlockChunk: UInt = 1000u,
    ftxSequencerSendingInterval: Duration = 100.milliseconds,
    ftxProcessingDelay: Duration = Duration.ZERO,
    fakeForcedTransactionsClientErrorRatio: Double = 0.5,
    safeBlockTracker: SafeBlockTracker? = null,
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
      invalidityProofClient = this.invalidityProofClient,
      stateManagerClient = this.stateManagerClient,
      accountProofClient = this.accountProofClient,
      tracesClient = this.tracesClient,
      clock = fakeClock,
      safeBlockNumberProvider = ForcedTransactionConflationSafeBlockNumberProvider(listener = safeBlockTracker),
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
  fun `should start with safe block number 0`() {
    val app = createApp()
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
    app.start().get()
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
    app.stop().get()
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
    val ftx11AddedEvent = ForcedTransactionAddedEvent.fromEthLog(ftxAddedEvents[2])
    assertThat(fxtDao.list().get()).isEqualTo(
      listOf(
        ForcedTransactionRecord(
          ftxNumber = 11UL,
          inclusionResult = ForcedTransactionInclusionResult.Included,
          simulatedExecutionBlockNumber = this.ftxClient.ftxInclusionResults[11UL]!!.blockNumber,
          simulatedExecutionBlockTimestamp = this.ftxClient.ftxInclusionResults[11UL]!!.blockTimestamp,
          ftxBlockNumberDeadline = ftx11AddedEvent.event.blockNumberDeadline,
          ftxRollingHash = ftx11AddedEvent.event.forcedTransactionRollingHash,
          ftxRlp = ftx11AddedEvent.event.rlpEncodedSignedTransaction,
          proofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
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
    app.stop().get()
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
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
      }

    // Verify no ftx were sent to the sequencer
    assertThat(ftxClient.ftxReceivedIds).isEmpty()
    // Verify no ftx records were created
    assertThat(fxtDao.list().get()).isEmpty()
    app.stop().get()
  }

  @Test
  fun `on restart shall wait to reach the tip of the l1 chain before releasing the conflation`() {
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

    val safeBlockTracker = SafeBlockTracker()
    val app = createApp(
      l1PollingInterval = 10.milliseconds,
      l1EventSearchBlockChunk = 100u, // Small chunks to simulate slow catch-up
      ftxSequencerSendingInterval = 5.milliseconds, // to empty the queue faster once it starts sending
      fakeForcedTransactionsClientErrorRatio = 0.0,
      safeBlockTracker = safeBlockTracker,
    )
    this.l1Client.setFinalizedBlockTag(20_000UL)
    this.l2Client.setLatestBlockTag(900UL)
    this.fakeClock.setTimeTo(this.l1Client.blockTimestamp(BlockParameter.Tag.LATEST) + 12.seconds)

    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
    app.start().get()
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(safeBlockTracker.stateTransitions.lastOrNull()).isEqualTo(900UL)
      }

    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 11UL,
      l2BlockNumber = 1011UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(safeBlockTracker.stateTransitions.lastOrNull()).isEqualTo(1011UL)
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
    // assert conflation is released after processing all FTXs
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(safeBlockTracker.stateTransitions.lastOrNull()).isNull()
      }
    assertThat(safeBlockTracker.stateTransitions).startsWith(
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
      ftxBlockNumberDeadline = 200UL,
      ftxRollingHash = ByteArray(0),
      ftxRlp = ByteArray(0),
      proofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
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
    app.stop().get()
  }

  @Test
  fun `should re-lock safe block number after contract upgrade from V7 to V8 as soon as 1st is sent to sequencer`() {
    // Scenario: Contract starts at V6/V7 and later on gets upgraded to V8.
    // forcedTransactionsUnsupportedYetByL1Contract() sets safeBlockNumber=null.
    // After awaitVersion() returns, the startup scan resumes.
    // The safe block number should be re-locked at 0 to prevent unrestricted conflation
    // while forced transactions are being discovered and sent to the sequencer.
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

    val safeBlockTracker = SafeBlockTracker()
    val app = createApp(
      l1PollingInterval = 10.milliseconds,
      l1EventSearchBlockChunk = 100u, // Small chunks to slow down catch-up
      ftxSequencerSendingInterval = 5.milliseconds,
      fakeForcedTransactionsClientErrorRatio = 0.0,
      safeBlockTracker = safeBlockTracker,
    )

    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

    val startFuture = app.start()
    // Upgrade to V8 so awaitVersion completes
    this.fakeContractClient.contractVersion = LineaRollupContractVersion.V8
    startFuture.get(5, TimeUnit.SECONDS)

    // it should lock safe block number with l2 chain head
    await()
      .atMost(10.seconds.toJavaDuration())
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
          null, // unlock while smart contract is not upgraded
          500UL, // lock on first ftx sent to sequencer
          510UL,
          520UL,
          null,
        )
      }

    app.stop().get()
  }

  @Test
  fun `should not hold the conflation while smart contract is not upgraded`() {
    this.fakeContractClient.contractVersion = LineaRollupContractVersion.V7
    val safeBlockTracker = SafeBlockTracker()
    val app = createApp(
      l1PollingInterval = 10.milliseconds,
      ftxSequencerSendingInterval = 100.milliseconds,
      safeBlockTracker = safeBlockTracker,
    )
    val startFuture = app.start()
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
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
      }

    assertThat(safeBlockTracker.stateTransitions).containsExactly(
      null, // unlock while smart contract is not upgraded
      10UL, // locked at L2 block number when processing FTX 1
      null, // released after processing FTX 1
    )
    app.stop().get()
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
      .atMost(10.seconds.toJavaDuration())
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
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(ftxClient.ftxReceivedIds).startsWith(1UL)
      }
    assertThat(ftxClient.ftxReceivedIds).doesNotContain(2UL)

    app.stop().get()
  }

  @Test
  fun `should trigger conflation and aggregation when sequencer processes ftxs`() {
    val ftxAddedEvents = listOf(
      createFtxAddedEvent(
        l1BlockNumber = 100UL,
        ftxNumber = 1UL,
        l2DeadLine = 100UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 101UL,
        ftxNumber = 2UL,
        l2DeadLine = 200UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 300UL,
        ftxNumber = 3UL,
        l2DeadLine = 300UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 400UL,
        ftxNumber = 4UL,
        l2DeadLine = 400UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 500UL,
        ftxNumber = 5UL,
        l2DeadLine = 500UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 600UL,
        ftxNumber = 6UL,
        l2DeadLine = 600UL,
      ),
      createFtxAddedEvent(
        l1BlockNumber = 700UL,
        ftxNumber = 7UL,
        l2DeadLine = 700UL,
      ),
    )
    this.l1Client.setLogs(ftxAddedEvents)
    this.fakeContractClient.finalizedStateProvider.l1FinalizedState = LineaRollupFinalizedState(
      blockNumber = 10UL,
      blockTimestamp = Clock.System.now(),
      messageNumber = 0UL,
      forcedTransactionNumber = 0UL,
    )

    val app = createApp(
      l1PollingInterval = 10.milliseconds,
      l1EventSearchBlockChunk = 100u,
      fakeForcedTransactionsClientErrorRatio = 0.0,
    )
    this.l1Client.setFinalizedBlockTag(1_000UL)
    this.l2Client.setLatestBlockTag(10UL)
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 1UL,
      l2BlockNumber = 100UL,
      inclusionResult = ForcedTransactionInclusionResult.BadNonce,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 2UL,
      l2BlockNumber = 101UL,
      inclusionResult = ForcedTransactionInclusionResult.BadNonce,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 3UL,
      l2BlockNumber = 130UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 4UL,
      l2BlockNumber = 140UL,
      inclusionResult = ForcedTransactionInclusionResult.Included,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 5UL,
      l2BlockNumber = 150UL,
      inclusionResult = ForcedTransactionInclusionResult.BadBalance,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 6UL,
      l2BlockNumber = 160UL,
      inclusionResult = ForcedTransactionInclusionResult.Phylax,
    )
    this.ftxClient.setFtxInclusionResultAfterReception(
      ftxNumber = 7UL,
      l2BlockNumber = 170UL,
      inclusionResult = ForcedTransactionInclusionResult.BadNonce,
    )
    this.fakeClock.setTimeTo(this.l1Client.blockTimestamp(BlockParameter.Tag.LATEST) + 12.seconds)
    val conflationTriggers = mutableListOf<Pair<BlockInterval, ConflationTrigger>>()
    val aggregationTriggers = mutableListOf<BlockInterval>()

    val conflationStartBlockNumberInclusive = 90UL
    val blobCompressor = FakeBlobCompressor(dataLimit = 1000)
    val conflationCalculator = ConflationCalculatorFactory.conflationCalculator(
      blobCompressor = blobCompressor,
      tracesCountersLimit = TracesCountersV5(TracingModuleV5.entries.associateWith { UInt.MAX_VALUE }),
      blocksLimit = 1000u,
      lastProcessedBlockNumber = conflationStartBlockNumberInclusive - 1UL,
      lastProcessedTimestamp = l2Client.genesisTimestamp,
      blobBatchesLimit = UInt.MAX_VALUE,
      aggregationProofsLimit = UInt.MAX_VALUE,
      dynamicBlockNumberSet = DynamicBlockNumberSet(),
      extraSyncCalculators = listOf(app.conflationCalculator),
      deferredTriggerConflationCalculators = emptyList(),
      metricsFacade = mock<MetricsFacade>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    )
    val aggregationCalculator = AggregationCalculatorFactory.createAggregationCalculator(
      startBlockNumberInclusive = conflationStartBlockNumberInclusive,
      maxProofsPerAggregation = UInt.MAX_VALUE,
      maxBlobsPerAggregation = UInt.MAX_VALUE,
      aggregationSizeMultipleOf = 6U,
      initialTimestamp = l2Client.blockTimestamp(1UL.toBlockParameter()),
      forcedTransactionTriggerAggCalculator = app.aggregationCalculator,
      metricsFacade = mock<MetricsFacade>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    )
    conflationCalculator.onConflatedBatch { conflation: ConflationCalculationResult ->
      conflationTriggers.add(
        BlockInterval(conflation.startBlockNumber, conflation.endBlockNumber) to conflation.conflationTrigger,
      )
      SafeFuture.completedFuture(Unit)
    }
    conflationCalculator.onBlobCreation { blob ->
      aggregationCalculator.newBlob(
        BlobCounters(
          numberOfBatches = 1u,
          startBlockNumber = blob.startBlockNumber,
          endBlockNumber = blob.endBlockNumber,
          startBlockTimestamp = blob.startBlockTime,
          endBlockTimestamp = blob.endBlockTime,
          expectedShnarf = ByteArray(0),
        ),
      )
      SafeFuture.completedFuture(Unit)
    }
    aggregationCalculator.onAggregation { agg ->
      aggregationTriggers.add(agg)
      SafeFuture.completedFuture(Unit)
    }
    app.start().get()

    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(this.fxtDao.list().get().lastOrNull()?.ftxNumber ?: 0UL).isGreaterThanOrEqualTo(1UL)
      }

    (conflationStartBlockNumberInclusive..190UL).forEach { blockNumber ->
      await()
        .pollInterval(1.milliseconds.toJavaDuration())
        .atMost(2.seconds.toJavaDuration())
        .untilAsserted {
          assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber() ?: 159UL)
            .isGreaterThanOrEqualTo(blockNumber.coerceAtMost(150UL))
        }
      val blockCounters = BlockCounters(
        blockNumber = blockNumber,
        blockTimestamp = Clock.System.now(),
        tracesCounters = TracesCountersV5.EMPTY_TRACES_COUNT,
        blockRLPEncoded = ByteArray(0),
      )
      conflationCalculator.newBlock(blockCounters)
    }

    assertThat(conflationTriggers).isEqualTo(
      listOf(
        BlockInterval(90UL, 99UL) to ConflationTrigger.FORCED_TRANSACTION,
        BlockInterval(100UL, 100UL) to ConflationTrigger.FORCED_TRANSACTION,
        BlockInterval(101UL, 129UL) to ConflationTrigger.FORCED_TRANSACTION,
        BlockInterval(130UL, 139UL) to ConflationTrigger.FORCED_TRANSACTION,
        BlockInterval(140UL, 149UL) to ConflationTrigger.FORCED_TRANSACTION,
      ),
    )
    assertThat(aggregationTriggers).isEqualTo(
      listOf(
        BlockInterval(90UL, 99UL),
        BlockInterval(100UL, 100UL),
        BlockInterval(101UL, 129UL),
        BlockInterval(130UL, 139UL),
        BlockInterval(140UL, 149UL),
      ),
    )
    assertThat(app.conflationSafeBlockNumberProvider.getHighestSafeBlockNumber())
      .isEqualTo(150UL)
    app.stop().get()
  }

  private fun EthApiBlockClient.blockTimestamp(blockParameter: BlockParameter): Instant =
    this.ethGetBlockByNumberTxHashes(blockParameter)
      .get().headerSummary.timestamp

  class SafeBlockTracker : SafeBlockNumberUpdateListener {
    val stateTransitions = CopyOnWriteArrayList<ULong?>()
    private var lastState: ULong? = ULong.MAX_VALUE // sentinel: will never match first update

    override fun onSafeBlockNumberUpdate(safeBlockNumber: ULong?) {
      if (lastState != safeBlockNumber) {
        lastState = safeBlockNumber
        stateTransitions.add(safeBlockNumber)
      }
    }
  }
}
