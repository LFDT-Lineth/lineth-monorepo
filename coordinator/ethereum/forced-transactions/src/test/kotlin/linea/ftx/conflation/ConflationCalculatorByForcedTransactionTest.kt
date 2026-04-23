package linea.ftx.conflation

import linea.conflation.calculators.ConflationCounters
import linea.conflation.calculators.ConflationTriggerCalculator
import linea.domain.BlockCounters
import linea.domain.ConflationTrigger
import linea.forcedtx.ForcedTransactionInclusionResult
import net.consensys.linea.traces.TracesCountersV4
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.EnumSource
import java.util.LinkedList
import java.util.Queue
import kotlin.time.Instant

class ConflationCalculatorByForcedTransactionTest {

  private lateinit var queue: Queue<FtxConflationInfo>
  private lateinit var calculator: ConflationCalculatorByForcedTransaction

  private val timestamp = Instant.parse("2024-01-01T00:00:00Z")
  private val ftxOverflowTrigger = ConflationTriggerCalculator.OverflowTrigger(
    trigger = ConflationTrigger.FORCED_TRANSACTION,
    singleBlockOverSized = false,
  )

  @BeforeEach
  fun setUp() {
    queue = LinkedList()
    calculator = ConflationCalculatorByForcedTransaction(queue)
  }

  private fun ftx(
    ftx: ULong,
    blockNumber: ULong,
    inclusionResult: ForcedTransactionInclusionResult,
  ) = FtxConflationInfo(
    ftxNumber = ftx,
    blockNumber = blockNumber,
    inclusionResult = inclusionResult,
  )

  private fun blockCounters(blockNumber: ULong) = BlockCounters(
    blockNumber = blockNumber,
    blockTimestamp = timestamp,
    tracesCounters = TracesCountersV4.EMPTY_TRACES_COUNT,
    blockRLPEncoded = ByteArray(0),
  )

  @Test
  fun `id should be FORCED_TRANSACTION`() {
    assertThat(calculator.id).isEqualTo(ConflationTrigger.FORCED_TRANSACTION.name)
  }

  @Test
  fun `appendBlock polls entries from the queue`() {
    queue.add(ftx(ftx = 1UL, blockNumber = 10UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce))

    calculator.appendBlock(blockCounters(blockNumber = 9UL))
    assertThat(queue).isNotEmpty // not polled yet
    calculator.appendBlock(blockCounters(blockNumber = 10UL))
    assertThat(queue).isEmpty()
  }

  @Test
  fun `copyCountersTo is a no-op`() {
    queue.add(ftx(ftx = 1UL, blockNumber = 10UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce))
    val counters = ConflationCounters(tracesCounters = TracesCountersV4.EMPTY_TRACES_COUNT)

    calculator.copyCountersTo(counters)

    assertThat(counters.blockCount).isEqualTo(0u)
    assertThat(counters.dataSize).isEqualTo(0u)
  }

  @Nested
  inner class CheckOverflow {
    @Test
    fun `returns null when the queue is empty`() {
      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 10UL))).isNull()
    }

    @ParameterizedTest(name = "inclusionResult={0}")
    @EnumSource(
      value = ForcedTransactionInclusionResult::class,
    )
    fun `triggers conflation at ftxBlockNumber for all non-Included results`(
      result: ForcedTransactionInclusionResult,
    ) {
      queue.add(ftx(ftx = 1UL, blockNumber = 10UL, inclusionResult = result))

      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 9UL))).isNull()
      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 10UL))).isEqualTo(ftxOverflowTrigger)
      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 11UL))).isNull()
    }

    @Test
    fun `triggers once per non-Included FTX at its respective boundary block`() {
      queue.add(ftx(ftx = 1UL, blockNumber = 10UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce))
      queue.add(ftx(ftx = 2UL, blockNumber = 20UL, inclusionResult = ForcedTransactionInclusionResult.BadBalance))

      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 10UL))).isEqualTo(ftxOverflowTrigger)
      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 20UL))).isEqualTo(ftxOverflowTrigger)
    }

    @Test
    fun `picks up FTXs added to the queue between calls`() {
      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 10UL))).isNull()

      queue.add(ftx(ftx = 1UL, blockNumber = 10UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce))

      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 10UL))).isEqualTo(ftxOverflowTrigger)
    }
  }

  @Nested
  inner class AppendBlock {

    @Test
    fun `clears pending triggers at or below the appended block number`() {
      queue.add(
        ftx(ftx = 1UL, blockNumber = 10UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce),
      ) // trigger at 10
      queue.add(
        ftx(ftx = 2UL, blockNumber = 20UL, inclusionResult = ForcedTransactionInclusionResult.BadBalance),
      ) // trigger at 20

      // Populate pendingTriggerBlocks: {10, 20}
      calculator.checkOverflow(blockCounters(blockNumber = 5UL))

      // Append block 10 - clears triggers <= 10 from pendingTriggerBlocks
      calculator.appendBlock(blockCounters(blockNumber = 10UL))

      // Clear queue so readProcessedFtxs won't re-add triggers on next checkOverflow
      queue.clear()

      // Trigger at 10 was cleared from pendingTriggerBlocks
      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 10UL))).isNull()
      // Trigger at 20 is still in pendingTriggerBlocks
      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 20UL))).isEqualTo(ftxOverflowTrigger)
    }

    @Test
    fun `does not clear pending triggers for future blocks`() {
      queue.add(
        ftx(ftx = 1UL, blockNumber = 20UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce),
      ) // trigger at 20

      calculator.checkOverflow(blockCounters(blockNumber = 5UL))
      calculator.appendBlock(blockCounters(blockNumber = 10UL))

      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 20UL))).isEqualTo(ftxOverflowTrigger)
    }
  }

  @Nested
  inner class Reset {

    @Test
    fun `does not clear pending trigger blocks`() {
      queue.add(ftx(ftx = 1UL, blockNumber = 10UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce))

      // Populate pendingTriggerBlocks
      calculator.checkOverflow(blockCounters(blockNumber = 5UL))
      // Remove from queue so the trigger can only come from pendingTriggerBlocks state
      queue.clear()

      calculator.reset()

      assertThat(calculator.checkOverflow(blockCounters(blockNumber = 10UL))).isEqualTo(ftxOverflowTrigger)
    }
  }
}
