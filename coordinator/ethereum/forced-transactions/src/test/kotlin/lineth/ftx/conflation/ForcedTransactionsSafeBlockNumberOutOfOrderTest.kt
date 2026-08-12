package lineth.ftx.conflation

import linea.contract.events.ForcedTransactionAddedEvent
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatCode
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Test
import kotlin.random.Random

/**
 * Deterministic reproduction of the out-of-order FTX crash observed in CI:
 *  - run 30330768989 (2026-07-28): ftx=4 at block 20 reported after ftx=3 at block 21
 *  - run 30917139702 (2026-08-04): ftx=5 at block 25 reported after ftx=4 at block 26
 */
class ForcedTransactionsSafeBlockNumberOutOfOrderTest {
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

  /** Replays the exact 2026-08-04 CI sequence (coordinator logs, lines 1518-1530). */
  private fun replayCiSequenceUpToTheCrashTrigger() {
    manager.caughtUpWithChainHeadAfterStartUp()
    // conflation was locked at block 23 before the batch was sent
    manager.lockSafeBlockNumberBeforeSendingToSequencer(23UL)
    // ftx=4 and ftx=5 sent to the sequencer concurrently
    manager.ftxSentToSequencer(listOf(createFtxEvent(4UL), createFtxEvent(5UL)))
    // sequencer placed ftx=4 at block 26 and ftx=5 at block 25;
    // ftx=4's result arrived first: safeBlockNumber 23 --> 26
    manager.ftxProcessedBySequencer(ftxNumber = 4UL, simulatedExecutionBlockNumber = 26UL)
    assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber()).isEqualTo(26UL)
  }

  @Test
  fun `reproduction - out-of-order ftx result throws and can never succeed on retry`() {
    replayCiSequenceUpToTheCrashTrigger()

    // ftx=5's result arrives with block 25 < safeBlockNumber 26 --> IllegalStateException
    assertThatThrownBy {
      manager.ftxProcessedBySequencer(ftxNumber = 5UL, simulatedExecutionBlockNumber = 25UL)
    }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("ftx=5")
      .hasMessageContaining("simulatedExecutionBlockNumber=25")
      .hasMessageContaining("safeBlockNumber=26")

    // The production failure mode: the caller retries the identical message every
    // ~5s; state never changes, so the retry is doomed and the lock is held forever.
    repeat(3) {
      assertThatThrownBy {
        manager.ftxProcessedBySequencer(ftxNumber = 5UL, simulatedExecutionBlockNumber = 25UL)
      }.isInstanceOf(IllegalStateException::class.java)
    }
    assertThat(safeBlockNumberProvider.getHighestSafeBlockNumber())
      .withFailMessage("lock is still held at 26 - finalization is permanently wedged")
      .isEqualTo(26UL)
  }
}
