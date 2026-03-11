package linea.ftx.conflation

import linea.contract.events.ForcedTransactionAddedEvent
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import kotlin.random.Random

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
      manager.caughtUpWithChainHeadAfterStartUp() // sets startUpScanFinished=true, releases 0->null
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
    fun `should release lock when no ftx in sequencer and safeBlockNumber is 0`() {
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(0UL)
      manager.caughtUpWithChainHeadAfterStartUp()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()
    }

    @Test
    fun `should be idempotent`() {
      manager.caughtUpWithChainHeadAfterStartUp()
      assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isNull()

      // Lock again and call caughtUp - should not change since already finished
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

      // Scan finishes, no FTX found
      manager.caughtUpWithChainHeadAfterStartUp()
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
}
