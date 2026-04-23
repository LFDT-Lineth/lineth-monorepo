package linea.ftx.conflation

import linea.conflation.calculators.AggregationTrigger
import linea.conflation.calculators.AggregationTriggerType
import linea.domain.BlobCounters
import linea.domain.BlobsToAggregate
import linea.forcedtx.ForcedTransactionInclusionResult
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.EnumSource
import java.util.LinkedList
import java.util.Queue
import kotlin.time.Instant

class AggregationCalculatorByForcedTransactionTest {

  private fun blobCounters(startBlockNumber: ULong, endBlockNumber: ULong) = BlobCounters(
    startBlockNumber = startBlockNumber,
    endBlockNumber = endBlockNumber,
    numberOfBatches = 1u,
    startBlockTimestamp = timestamp,
    endBlockTimestamp = timestamp,
    expectedShnarf = ByteArray(32),
  )

  private lateinit var queue: Queue<FtxConflationInfo>
  private lateinit var calculator: AggregationCalculatorByForcedTransaction

  private val timestamp = Instant.parse("2024-01-01T00:00:00Z")

  @BeforeEach
  fun setUp() {
    queue = LinkedList()
    calculator = AggregationCalculatorByForcedTransaction(queue)
  }

  private fun ftx(
    ftxNumber: ULong,
    blockNumber: ULong,
    inclusionResult: ForcedTransactionInclusionResult = ForcedTransactionInclusionResult.BadNonce,
  ) = FtxConflationInfo(
    ftxNumber = ftxNumber,
    blockNumber = blockNumber,
    inclusionResult = inclusionResult,
  )

  private fun expectedTrigger(endBlockNumber: ULong) = AggregationTrigger(
    aggregationTriggerType = AggregationTriggerType.FORCED_TRANSACTION,
    aggregation = BlobsToAggregate(startBlockNumber = endBlockNumber, endBlockNumber = endBlockNumber),
  )

  @Test
  fun `returns null when queue is empty`() {
    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 1UL, endBlockNumber = 10UL)))
      .isNull()
  }

  @ParameterizedTest(name = "inclusionResult={0}")
  @EnumSource(ForcedTransactionInclusionResult::class)
  fun `triggers at (ftxBlockNumber - 1) for all inclusion result types`(result: ForcedTransactionInclusionResult) {
    // FTX at block 10 → aggregation target = 9 (= 10 - 1)
    // The aggregation seals at block 9, making FTX the first block of the next aggregation.
    queue.add(ftx(ftxNumber = 1UL, blockNumber = 10UL, inclusionResult = result))

    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 8UL, endBlockNumber = 8UL)))
      .isNull()
    // Trigger fires when blob ending at 9 (= ftxBlock - 1) is processed
    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 9UL, endBlockNumber = 9UL)))
      .isEqualTo(expectedTrigger(9UL))
    calculator.reset()
    // No trigger for the FTX block itself or beyond
    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 10UL, endBlockNumber = 10UL)))
      .isNull()
  }

  @Test
  fun `consumes FTX from queue when processing trigger`() {
    // FTX at block 10 → trigger fires when blob ending at 9 is processed
    queue.add(ftx(ftxNumber = 1UL, blockNumber = 10UL))

    calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 9UL, endBlockNumber = 9UL))

    assertThat(queue).isEmpty()
  }

  @Test
  fun `multiple FTXs each trigger at (ftxBlockNumber - 1)`() {
    queue.add(ftx(ftxNumber = 1UL, blockNumber = 10UL)) // target: 9
    queue.add(ftx(ftxNumber = 2UL, blockNumber = 20UL)) // target: 19
    queue.add(ftx(ftxNumber = 3UL, blockNumber = 30UL)) // target: 29

    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 9UL, endBlockNumber = 9UL)))
      .isEqualTo(expectedTrigger(9UL))
    calculator.reset()
    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 10UL, endBlockNumber = 18UL)))
      .isNull()
    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 19UL, endBlockNumber = 19UL)))
      .isEqualTo(expectedTrigger(19UL))
    calculator.reset()
    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 29UL, endBlockNumber = 29UL)))
      .isEqualTo(expectedTrigger(29UL))
  }

  @Test
  fun `timing guarantee - FTX added to queue between calls is detected on next call`() {
    // Blob [8..8] processed: FTX not in queue yet
    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 8UL, endBlockNumber = 8UL)))
      .isNull()

    // FTX result arrives (guaranteed by safeBlockNumber: added to queue before blob [9..9] can be proven)
    // FTX at block 10 → aggregation target = 9
    queue.add(ftx(ftxNumber = 1UL, blockNumber = 10UL))

    // Blob [9..9]: target=9 → trigger fires, sealing aggregation before FTX@10
    assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 9UL, endBlockNumber = 9UL)))
      .isEqualTo(expectedTrigger(9UL))
  }

  @Nested
  inner class Reset {
    @Test
    fun `pending trigger blocks survive reset so future blobs can match`() {
      // FTX at block 10 → target = 9
      queue.add(ftx(ftxNumber = 1UL, blockNumber = 10UL))

      // Populate trigger blocks by processing an earlier blob
      calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 5UL, endBlockNumber = 5UL))
      queue.clear() // ensure trigger only comes from pendingTriggerBlocks state, not re-read from queue

      calculator.reset()

      assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 9UL, endBlockNumber = 9UL)))
        .isEqualTo(expectedTrigger(9UL))
    }

    @Test
    fun `reset consumes remaining FTXs from queue into pending trigger blocks`() {
      // FTX at block 10 → target = 9
      queue.add(ftx(ftxNumber = 1UL, blockNumber = 10UL))

      calculator.reset() // should consume FTX and add target at 9

      assertThat(queue).isEmpty()
      assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 9UL, endBlockNumber = 9UL)))
        .isEqualTo(expectedTrigger(9UL))
    }
  }

  @Nested
  inner class ProverConstraintGuarantee {
    /**
     * Each FTX execution block is the FIRST block of its aggregation.
     * The aggregation seals at (ftxBlock - 1), so the new aggregation starts at the FTX block.
     * This guarantees each aggregation contains at most one FTX execution block,
     * satisfying the prover constraint that all invalidity proofs in an aggregation
     * share the same SimulatedExecutionBlockNumber.
     */
    @Test
    fun `consecutive FTX blocks each seal their own aggregation`() {
      // 4 FTXs at consecutive blocks → aggregation targets at {31, 32, 33, 34}
      queue.add(ftx(ftxNumber = 1UL, blockNumber = 32UL, inclusionResult = ForcedTransactionInclusionResult.BadNonce))
      queue.add(
        ftx(ftxNumber = 2UL, blockNumber = 33UL, inclusionResult = ForcedTransactionInclusionResult.BadPrecompile),
      )
      queue.add(ftx(ftxNumber = 3UL, blockNumber = 34UL, inclusionResult = ForcedTransactionInclusionResult.Included))
      queue.add(
        ftx(ftxNumber = 4UL, blockNumber = 35UL, inclusionResult = ForcedTransactionInclusionResult.TooManyLogs),
      )

      // Blob [31..31]: seals aggregation before FTX-1@32
      assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 31UL, endBlockNumber = 31UL)))
        .isEqualTo(expectedTrigger(31UL))
      calculator.reset()

      // Blob [32..32]: seals aggregation before FTX-2@33 (aggregation [32..32] contains only FTX-1)
      assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 32UL, endBlockNumber = 32UL)))
        .isEqualTo(expectedTrigger(32UL))
      calculator.reset()

      // Blob [33..33]: seals aggregation before FTX-3@34
      assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 33UL, endBlockNumber = 33UL)))
        .isEqualTo(expectedTrigger(33UL))
      calculator.reset()

      // Blob [34..34]: seals aggregation before FTX-4@35
      assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 34UL, endBlockNumber = 34UL)))
        .isEqualTo(expectedTrigger(34UL))
    }

    @Test
    fun `blob ending just before FTX block triggers aggregation`() {
      // FTX at block 32 → aggregation target = 31
      queue.add(ftx(ftxNumber = 1UL, blockNumber = 32UL))

      // Blob spanning several non-FTX blocks, ending just before FTX
      assertThat(calculator.checkAggregationTrigger(blobCounters(startBlockNumber = 28UL, endBlockNumber = 31UL)))
        .isEqualTo(
          AggregationTrigger(
            aggregationTriggerType = AggregationTriggerType.FORCED_TRANSACTION,
            aggregation = BlobsToAggregate(startBlockNumber = 28UL, endBlockNumber = 31UL),
          ),
        )
    }
  }
}
