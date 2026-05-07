package linea.ftx.conflation

import linea.contract.events.ForcedTransactionAddedEvent
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.ftx.FakeForcedTransactionsClient
import linea.ftx.ForcedTransactionWithTimestamp
import linea.ftx.ForcedTransactionsStatusUpdater
import linea.persistence.ftx.FakeForcedTransactionsDao
import net.consensys.FakeFixedClock
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import java.util.Queue
import java.util.concurrent.LinkedBlockingQueue
import kotlin.random.Random
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Instant

class ForcedTransactionsSafeBlockNumberManagerTest {
  private lateinit var manager: ForcedTransactionsSafeBlockNumberManager
  private lateinit var safeBlockNumberProvider: ForcedTransactionConflationSafeBlockNumberProvider

  private fun createFtxEvent(ftxNumber: ULong): ForcedTransactionAddedEvent {
    return ForcedTransactionAddedEvent(
      forcedTransactionNumber = ftxNumber,
      from = Random.nextBytes(20),
      blockNumberDeadline = 1000UL,
      forcedTransactionRollingHash = Random.nextBytes(32),
      rlpEncodedSignedTransaction = Random.nextBytes(64),
    )
  }

  @BeforeEach
  fun setUp() {
    safeBlockNumberProvider = ForcedTransactionConflationSafeBlockNumberProvider()
    manager = ForcedTransactionsSafeBlockNumberManager(listener = safeBlockNumberProvider)
  }

  @Test
  fun `initial state should be locked at 0`() {
    assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
  }

  @Nested
  inner class LockSafeBlockNumberBeforeSendingToSequencer {
    @Test
    fun `should lock to headBlockNumber when startup finished and safeBlockNumber is null`() {
      // caughtUp now only marks startup finished; the release happens via unprocessedFtxQueueIsEmpty
      manager.caughtUpWithChainHeadAfterStartUp()
      manager.unprocessedFtxQueueIsEmpty()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()

      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)
    }

    @Test
    fun `should not change when already locked at a block number greater than 0`() {
      manager.caughtUpWithChainHeadAfterStartUp()
      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)

      manager.lockSafeBlockNumberBeforeSendingToSequencer(600UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)
    }
  }

  @Nested
  inner class FtxProcessedBySequencer {
    @Test
    fun `should set safeBlockNumber to simulatedExecutionBlockNumber when ftx remain in sequencer`() {
      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      manager.ftxSentToSequencer(listOf(createFtxEvent(1UL), createFtxEvent(2UL)))
      manager.ftxProcessedBySequencer(1UL, 600UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(600UL)
    }
  }

  @Nested
  inner class UnprocessedFtxQueueIsEmpty {
    @Test
    fun `should not release lock when startup scan not finished`() {
      manager.unprocessedFtxQueueIsEmpty()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
    }

    @Test
    fun `should release lock when startup scan finished`() {
      manager.caughtUpWithChainHeadAfterStartUp()
      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)

      manager.unprocessedFtxQueueIsEmpty()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }
  }

  @Nested
  inner class CaughtUpWithChainHeadAfterStartUp {
    @Test
    fun `does not release the lock on its own`() {
      // caughtUp only marks the L1 startup scan as finished; it must not release the lock,
      // because FTXs already queued by the fetcher may still be awaiting their sequencer status.
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
      manager.caughtUpWithChainHeadAfterStartUp()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

      // The actual release happens once the L1-events queue is genuinely drained
      // (the call site of unprocessedFtxQueueIsEmpty enforces ftxQueue.isEmpty()).
      manager.unprocessedFtxQueueIsEmpty()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }

    @Test
    fun `is idempotent`() {
      manager.caughtUpWithChainHeadAfterStartUp()
      manager.unprocessedFtxQueueIsEmpty()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()

      // Lock at a head and call caughtUp again - it must remain a no-op
      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)
      manager.caughtUpWithChainHeadAfterStartUp()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)
    }
  }

  @Nested
  inner class ForcedTransactionsUnsupportedYetByL1Contract {
    @Test
    fun `should release lock`() {
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
      manager.forcedTransactionsUnsupportedYetByL1Contract()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }
  }

  @Nested
  inner class StartupScenarios {
    @Test
    fun `normal startup with no FTX on L1 should release lock after catching up`() {
      // Start: safeBlockNumber=0, startUpScanFinished=false
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

      // Scan finishes, no FTX found. caughtUp only marks startup finished;
      // the StatusUpdater's next tick observes an empty queue and releases the lock.
      manager.caughtUpWithChainHeadAfterStartUp()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

      manager.unprocessedFtxQueueIsEmpty()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }

    @Test
    fun `normal startup with FTX discovered during scan`() {
      // Start: safeBlockNumber=0
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

      // FTX sent to sequencer during scan (lockSafeBlockNumberBeforeSendingToSequencer
      // would early-return since !startUpScanFinished, but safeBlockNumber=0 keeps conflation locked)
      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)

      manager.ftxSentToSequencer(listOf(createFtxEvent(1UL)))

      // Scan catches up
      manager.caughtUpWithChainHeadAfterStartUp()
      // FTX still in sequencer, so lock is kept
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)

      // FTX processed
      manager.ftxProcessedBySequencer(1UL, 510UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }

    @Test
    fun `normal startup with FTXs discovered and processed during scan`() {
      // Start: safeBlockNumber=0
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

      // FTX sent to sequencer during scan (lockSafeBlockNumberBeforeSendingToSequencer
      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)

      manager.ftxSentToSequencer(listOf(createFtxEvent(1UL)))
      manager.ftxSentToSequencer(listOf(createFtxEvent(2UL)))
      manager.ftxProcessedBySequencer(1UL, 550UL)

      // Scan catches up
      manager.caughtUpWithChainHeadAfterStartUp()
      // still on unprocessed FTX, so lock is kept ftx1 simulated block number is 550
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(550UL)

      // FTX 2 processed
      manager.ftxProcessedBySequencer(2UL, 560UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }

    @Test
    fun `V7 to V8 upgrade - safeBlockNumber stays null during scan`() {
      // Start: safeBlockNumber=0
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

      // Contract is V7 - release lock
      manager.forcedTransactionsUnsupportedYetByL1Contract()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()

      // V8 upgrade happens, scan starts
      // lockSafeBlockNumberBeforeSendingToSequencer early-returns since !startUpScanFinished
      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)
    }

    @Test
    fun `V7 to V8 upgrade - caughtUp does not re-lock after forcedTransactionsUnsupported`() {
      // Start: safeBlockNumber=0
      manager.forcedTransactionsUnsupportedYetByL1Contract()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()

      // Even after caughtUp, safeBlockNumber stays null because it's not 0UL
      manager.caughtUpWithChainHeadAfterStartUp()
      // The check `if (safeBlockNumber == 0UL)` is false since safeBlockNumber is null
      // so it doesn't release (but it was already null, so effectively the same)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }

    @Test
    fun `V7 to V8 upgrade with FTX sent during scan - conflation unrestricted`() {
      // Start: safeBlockNumber=0
      manager.forcedTransactionsUnsupportedYetByL1Contract()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()

      // FTX discovered and sent during scan
      manager.lockSafeBlockNumberBeforeSendingToSequencer(500UL)
      manager.ftxSentToSequencer(listOf(createFtxEvent(1UL)))
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)

      // After scan catches up
      manager.caughtUpWithChainHeadAfterStartUp()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(500UL)
    }
  }

  /**
   * Regression scenarios reproducing the wedge observed on CI: after a coordinator restart,
   * the L1 fetcher caught up with FTXs already queued, the safe-block lock was released to
   * null, conflation advanced past the FTX execution block and the resulting aggregation
   * proof carried `finalFtxNumber=0` — leading to a permanent `FinalizationStateIncorrect`
   * revert on the next aggregation's finalize.
   *
   * These tests exercise the manager + StatusUpdater + ftxQueue together because the fix
   * spans both the manager (caughtUp no longer releases) and the StatusUpdater call site
   * (release gated on ftxQueue.isEmpty()).
   */
  @Nested
  inner class StartupRaceWithQueuedFtx {
    private val now = Instant.parse("2026-04-29T18:50:30Z")
    private val l1BlockTimestamp = Instant.parse("2026-04-29T18:50:15Z")

    private lateinit var clock: FakeFixedClock
    private lateinit var dao: FakeForcedTransactionsDao
    private lateinit var ftxClient: FakeForcedTransactionsClient
    private lateinit var ftxQueue: Queue<ForcedTransactionWithTimestamp>
    private lateinit var processedFtx: MutableList<FtxConflationInfo>

    private fun newStatusUpdater(
      ftxProcessingDelay: Duration = Duration.ZERO,
    ): ForcedTransactionsStatusUpdater {
      return ForcedTransactionsStatusUpdater(
        dao = dao,
        ftxClient = ftxClient,
        safeBlockNumberManager = manager,
        ftxQueue = ftxQueue,
        ftxProcessedListener = { processedFtx.add(it) },
        lastProcessedFtxNumber = 0UL,
        ftxProcessingDelay = ftxProcessingDelay,
        clock = clock,
      )
    }

    @BeforeEach
    fun setUpStatusUpdaterFixtures() {
      clock = FakeFixedClock(now)
      dao = FakeForcedTransactionsDao()
      ftxClient = FakeForcedTransactionsClient()
      ftxQueue = LinkedBlockingQueue()
      processedFtx = mutableListOf()
    }

    /**
     * Direct reproduction of the wedge: before the fix, [caughtUpWithChainHeadAfterStartUp]
     * released the lock to null whenever `safeBlockNumber == 0UL` and the sequencer hadn't
     * yet been told about any FTX, ignoring the L1-events queue.
     */
    @Test
    fun `caughtUpWithChainHeadAfterStartUp does not release the lock by itself`() {
      // L1 fetcher has just enqueued an FTX before signalling that it has caught up
      ftxQueue.add(ForcedTransactionWithTimestamp(createFtxEvent(1UL), l1BlockTimestamp))

      manager.caughtUpWithChainHeadAfterStartUp()

      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber())
        .describedAs("lock must remain held while ftxQueue holds an unprocessed FTX")
        .isEqualTo(0UL)
    }

    /**
     * Tests the StatusUpdater call-site guard: `unprocessedFtxQueueIsEmpty()` must only fire
     * when the L1-events queue is genuinely drained. When an FTX is still inside
     * `ftxProcessingDelay`, the filtered list returned by `getUnprocessedForcedTransactions`
     * is empty, but the queue itself is not. Releasing the lock in that state would let
     * conflation race past the FTX block.
     */
    @Test
    fun `unprocessedFtxQueueIsEmpty does not fire while a queued FTX is inside the processing delay`() {
      ftxQueue.add(ForcedTransactionWithTimestamp(createFtxEvent(1UL), now))
      val statusUpdater = newStatusUpdater(ftxProcessingDelay = 1.minutes)

      manager.caughtUpWithChainHeadAfterStartUp()
      val unprocessed = statusUpdater.getUnprocessedForcedTransactions().get()

      assertThat(unprocessed).isEmpty()
      assertThat(ftxQueue).hasSize(1)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber())
        .describedAs("lock must be held while the queue still contains an FTX")
        .isEqualTo(0UL)
    }

    /**
     * Once the sequencer confirms the queued FTX, the StatusUpdater drains the queue and
     * the manager releases the lock through the normal `ftxProcessedBySequencer` path.
     */
    @Test
    fun `lock is released after the sequencer confirms the queued FTX`() {
      ftxQueue.add(ForcedTransactionWithTimestamp(createFtxEvent(1UL), l1BlockTimestamp))
      val statusUpdater = newStatusUpdater()

      manager.caughtUpWithChainHeadAfterStartUp()
      // Sequencer not yet aware of FTX#1
      statusUpdater.getUnprocessedForcedTransactions().get()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
      assertThat(ftxQueue).hasSize(1)

      // Sequencer publishes status -> next poll drains the queue
      ftxClient.setFtxInclusionResult(
        ftxNumber = 1UL,
        l2BlockNumber = 14UL,
        inclusionResult = ForcedTransactionInclusionResult.BadNonce,
      )
      statusUpdater.getUnprocessedForcedTransactions().get()

      assertThat(ftxQueue).isEmpty()
      assertThat(processedFtx).hasSize(1)
      assertThat(processedFtx.first().ftxNumber).isEqualTo(1UL)
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber())
        .describedAs("lock must be released once the queue drains end-to-end")
        .isNull()
    }

    /**
     * No FTX ever submitted: the L1 scan catches up and the next poll observes both
     * the queue and the filtered list empty, so the lock is released to null.
     */
    @Test
    fun `lock is released on first poll after caught-up when no FTXs were ever queued`() {
      val statusUpdater = newStatusUpdater()

      manager.caughtUpWithChainHeadAfterStartUp()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)

      statusUpdater.getUnprocessedForcedTransactions().get()

      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }

    /**
     * Polls happening before the L1 scan finishes must not release the lock,
     * even when both the queue and the filtered list are empty.
     */
    @Test
    fun `lock is held while L1 startup scan has not finished`() {
      val statusUpdater = newStatusUpdater()
      statusUpdater.getUnprocessedForcedTransactions().get()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
    }
  }
}
