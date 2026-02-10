package net.consensys.zkevm.ethereum.coordination.aggregation

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import linea.domain.BlockHeaderSummary
import linea.kotlin.ByteArrayExt
import net.consensys.FakeFixedClock
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import org.assertj.core.api.Assertions
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertDoesNotThrow
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import org.mockito.Mockito
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.stream.Stream
import kotlin.IllegalArgumentException
import kotlin.random.Random
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class GlobalAggregationCalculatorTest {
  private lateinit var fixedClock: FakeFixedClock
  private lateinit var safeBlockProvider: SafeBlockProvider
  private lateinit var aggregationTriggerCalculatorByDeadline: AggregationTriggerCalculatorByDeadline
  private lateinit var aggregationTriggerCalculatorByTimestampHardFork: AggregationTriggerCalculatorByTimestampHardFork

  @BeforeEach
  fun setup() {
    fixedClock = FakeFixedClock()
    safeBlockProvider = mock()
  }

  private fun aggregationCalculator(
    lastBlockNumber: ULong = 0u,
    proofLimit: UInt? = null,
    blobLimit: UInt? = null,
    aggregationDeadline: Duration? = null,
    aggregationDeadlineDelay: Duration? = aggregationDeadline?.div(2),
    targetBlockNumbers: List<Int>? = null,
    hardForkTimestamps: List<Instant>? = null,
    initialTimestamp: Instant = fixedClock.now(),
    aggregationSizeMultipleOf: Int = 1,
    metricsFacade: MetricsFacade = mock<MetricsFacade>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    aggregationHandler: AggregationHandler = AggregationHandler.NOOP_HANDLER,
  ): GlobalAggregationCalculator {
    val syncAggregationTriggers = mutableListOf<SyncAggregationTriggerCalculator>()
      .apply {
        proofLimit?.also { add(AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = it)) }
        blobLimit?.also { add(AggregationTriggerCalculatorByBlobLimit(maxBlobsPerAggregation = it)) }
        targetBlockNumbers?.also {
          add(AggregationTriggerCalculatorByTargetBlockNumbers(targetBlockNumbers.map { it.toULong() }))
        }
        hardForkTimestamps?.also {
          aggregationTriggerCalculatorByTimestampHardFork = AggregationTriggerCalculatorByTimestampHardFork(
            hardForkTimestamps = it,
            initialTimestamp = initialTimestamp,
          )
          add(aggregationTriggerCalculatorByTimestampHardFork)
        }
      }

    val deferredAggregationTriggers = mutableListOf<DeferredAggregationTriggerCalculator>().apply {
      aggregationDeadline?.also {
        aggregationTriggerCalculatorByDeadline = AggregationTriggerCalculatorByDeadline(
          AggregationTriggerCalculatorByDeadline.Config(
            aggregationDeadline = aggregationDeadline,
            noL2ActivityTimeout = aggregationDeadlineDelay!!,
            waitForNoL2ActivityToTriggerAggregation = true,
          ),
          fixedClock,
          safeBlockProvider,
        )
        add(aggregationTriggerCalculatorByDeadline)
      }
    }

    return GlobalAggregationCalculator(
      lastBlockNumber = lastBlockNumber,
      syncAggregationTrigger = syncAggregationTriggers,
      deferredAggregationTrigger = deferredAggregationTriggers,
      metricsFacade = metricsFacade,
      aggregationSizeMultipleOf = aggregationSizeMultipleOf.toUInt(),
    ).apply { onAggregation(aggregationHandler) }
  }

  private fun blobCounters(
    startBlockNumber: ULong,
    endBlockNumber: ULong,
    numberOfBatches: UInt = 1u,
    startBlockTimestamp: Instant = fixedClock.now(),
    endBlockTimestamp: Instant = fixedClock.now().plus(2.seconds.times((endBlockNumber - startBlockNumber).toInt())),
  ): BlobCounters {
    return BlobCounters(
      numberOfBatches = numberOfBatches,
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      startBlockTimestamp = startBlockTimestamp,
      endBlockTimestamp = endBlockTimestamp,
      expectedShnarf = Random.nextBytes(32),
    )
  }

  @Test
  fun when_out_of_order_blob_then_throw_exception() {
    val globalAggregationCalculator = aggregationCalculator(proofLimit = 15u)
    val expectedErrorMessage = "Blobs to aggregate must be sequential: lastBlockNumber=0, startBlockNumber=2 for " +
      "new blob"
    Assertions.assertThatThrownBy {
      globalAggregationCalculator.newBlob(
        blobCounters(
          numberOfBatches = 5u,
          startBlockNumber = 2u,
          endBlockNumber = 10u,
        ),
      )
    }
      .message()
      .isEqualTo(expectedErrorMessage)
  }

  @Test
  fun when_one_blob_exceeds_proof_limit_then_throw_exception() {
    val maxProofsPerAggregation = 15u
    val globalAggregationCalculator = aggregationCalculator(proofLimit = maxProofsPerAggregation)
    val expectedErrorMessage = "Number of proofs in one blob exceed the aggregation proof limit"
    val exception = assertThrows<IllegalArgumentException> {
      globalAggregationCalculator.newBlob(
        blobCounters(
          numberOfBatches = maxProofsPerAggregation,
          startBlockNumber = 1u,
          endBlockNumber = 10u,

        ),
      )
    }
    assertThat(exception.message).contains(expectedErrorMessage)
  }

  @Test
  fun when_new_blob_proofs_equals_proof_limit_verify_aggregation() {
    val proofLimit = 15u

    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val expectedAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(proofLimit = proofLimit) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // This blob with #proofs = 6 does not trigger aggregation. ProofCount after adding this blob is 6
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 5u,
        startBlockNumber = 1u,
        endBlockNumber = 10u,
      ),
    )

    // This blob with #proofs = proofLimit (15) will trigger aggregation 2 times, first for the previous blob and
    // second for itself. Proof count after adding this blob is 0
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = proofLimit - 1u,
        startBlockNumber = 11u,
        endBlockNumber = 30u,
      ),
    )
    expectedAggregations.add(BlobsToAggregate(1u, 10u))
    expectedAggregations.add(BlobsToAggregate(11u, 30u))

    // This blob with #proofs = proofLimit will trigger aggregation. ProofCount after adding this blob is 0
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = proofLimit - 1u,
        startBlockNumber = 31u,
        endBlockNumber = 45u,
      ),
    )
    expectedAggregations.add(BlobsToAggregate(31u, 45u))

    // This blob with #proofs = 5 will not trigger aggregation. ProofCount after adding this blob is equal to 5
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 4u,
        startBlockNumber = 46u,
        endBlockNumber = 61u,
      ),
    )

    // This blob with #proofs = 10 will trigger aggregation and will be included in aggregation along with previous
    // blob. ProofCount is zero after this blob
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 9u,
        startBlockNumber = 62u,
        endBlockNumber = 70u,
      ),
    )
    expectedAggregations.add(BlobsToAggregate(46u, 70u))

    // This blob with #proofs = proofLimit will trigger aggregation and will be included in aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = proofLimit - 1u,
        startBlockNumber = 71u,
        endBlockNumber = 85u,
      ),
    )
    expectedAggregations.add(BlobsToAggregate(71u, 85u))
    assertThat(actualAggregations).containsExactlyElementsOf(expectedAggregations)
  }

  @Test
  fun when_new_blobs_exceed_proof_limit_verify_trigger_type_and_aggregation() {
    val proofsLimit = 15u
    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val expectedAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(proofLimit = proofsLimit) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // This blob with 6 proofs does not trigger aggregation. Proof count after adding this blob is 6
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 5u,
        startBlockNumber = 1u,
        endBlockNumber = 10u,
      ),
    )

    // This blob with proof count 13 will trigger aggregation, but will not be included in any aggregation.
    // Proof count after adding this will be 13
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 12u,
        startBlockNumber = 11u,
        endBlockNumber = 30u,
      ),
    )
    expectedAggregations.add(BlobsToAggregate(1u, 10u))

    // This new blob with 10 proofs will trigger aggregation but not included in it.
    // Proof count after adding this blob is 10
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 9u,
        startBlockNumber = 31u,
        endBlockNumber = 45u,
      ),
    )

    expectedAggregations.add(BlobsToAggregate(11u, 30u))

    // This blob with 5 proofs will trigger aggregation including the previous blob and this one.
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 4u,
        startBlockNumber = 46u,
        endBlockNumber = 61u,
      ),
    )
    expectedAggregations.add(BlobsToAggregate(31u, 61u))
    assertThat(actualAggregations).containsExactlyElementsOf(expectedAggregations)
  }

  @Test
  fun when_blob_limit_reached_verify_aggregation() {
    val blobLimit = 3u
    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(blobLimit = blobLimit) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // Add first blob - should not trigger aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 2u,
        startBlockNumber = 1u,
        endBlockNumber = 5u,
      ),
    )
    assertThat(actualAggregations).isEmpty()

    // Add second blob - should not trigger aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 6u,
        endBlockNumber = 10u,
      ),
    )
    assertThat(actualAggregations).isEmpty()

    // Add third blob - should trigger aggregation of all 3 blobs
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 1u,
        startBlockNumber = 11u,
        endBlockNumber = 15u,
      ),
    )

    assertThat(actualAggregations).hasSize(1)
    assertThat(actualAggregations[0]).isEqualTo(BlobsToAggregate(1u, 15u))
  }

  @Test
  fun when_blob_limit_exceeded_verify_multiple_aggregations() {
    val blobLimit = 2u
    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(blobLimit = blobLimit) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // Add first blob
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 1u,
        startBlockNumber = 1u,
        endBlockNumber = 5u,
      ),
    )

    // Add second blob - should trigger aggregation of first 2 blobs
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 2u,
        startBlockNumber = 6u,
        endBlockNumber = 10u,
      ),
    )

    assertThat(actualAggregations).hasSize(1)
    assertThat(actualAggregations[0]).isEqualTo(BlobsToAggregate(1u, 10u))

    // Add third blob
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 1u,
        startBlockNumber = 11u,
        endBlockNumber = 15u,
      ),
    )

    // Add fourth blob - should trigger aggregation of blobs 3 and 4
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 2u,
        startBlockNumber = 16u,
        endBlockNumber = 20u,
      ),
    )

    assertThat(actualAggregations).hasSize(2)
    assertThat(actualAggregations[1]).isEqualTo(BlobsToAggregate(11u, 20u))
  }

  @Test
  fun when_blob_limit_with_single_blob_aggregation() {
    val blobLimit = 1u
    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(blobLimit = blobLimit) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // Each blob should trigger immediate aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 2u,
        startBlockNumber = 1u,
        endBlockNumber = 5u,
      ),
    )

    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 6u,
        endBlockNumber = 10u,
      ),
    )

    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 1u,
        startBlockNumber = 11u,
        endBlockNumber = 15u,
      ),
    )

    assertThat(actualAggregations).hasSize(3)
    assertThat(actualAggregations[0]).isEqualTo(BlobsToAggregate(1u, 5u))
    assertThat(actualAggregations[1]).isEqualTo(BlobsToAggregate(6u, 10u))
    assertThat(actualAggregations[2]).isEqualTo(BlobsToAggregate(11u, 15u))
  }

  @Test
  fun when_blob_limit_combined_with_proof_limit_verify_aggregation() {
    val blobLimit = 3u
    val proofLimit = 50u
    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(
      blobLimit = blobLimit,
      proofLimit = proofLimit,
    ) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // Add first blob with 41 proofs - should not trigger aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 40u,
        startBlockNumber = 1u,
        endBlockNumber = 5u,
      ),
    )
    assertThat(actualAggregations).isEmpty()

    // Add second blob with 30 proofs (total 72) - should trigger aggregation by proof limit
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 30u,
        startBlockNumber = 6u,
        endBlockNumber = 10u,
      ),
    )

    assertThat(actualAggregations).hasSize(1)
    assertThat(actualAggregations[0]).isEqualTo(BlobsToAggregate(1u, 5u)) // Only first blob aggregated

    // Continue with third blob - should not trigger aggregation yet
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 1u,
        startBlockNumber = 11u,
        endBlockNumber = 15u,
      ),
    )

    // Fourth blob should trigger aggregation by blob limit (3 blobs accumulated)
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 1u,
        startBlockNumber = 16u,
        endBlockNumber = 20u,
      ),
    )

    assertThat(actualAggregations).hasSize(2)
    assertThat(actualAggregations[1]).isEqualTo(BlobsToAggregate(6u, 20u))
  }

  @Test
  fun `metrics are exported correctly when aggregation is triggered by blob limit`() {
    val testMeterRegistry = SimpleMeterRegistry()
    val globalAggregationCalculator = aggregationCalculator(
      blobLimit = 2u,
      metricsFacade = MicrometerMetricsFacade(testMeterRegistry, "test"),
    )
    val pendingProofsGauge = testMeterRegistry.get("test.aggregation.proofs.ready").gauge()
    assertThat(pendingProofsGauge.value()).isEqualTo(0.0)

    globalAggregationCalculator.onAggregation {
      SafeFuture.completedFuture(Unit)
    }

    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 1u,
        endBlockNumber = 10u,
      ),
    )
    assertThat(pendingProofsGauge.value()).isEqualTo(4.0)

    // This blob should cause aggregation by blob limit
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 2u,
        startBlockNumber = 11u,
        endBlockNumber = 20u,
      ),
    )
    assertThat(pendingProofsGauge.value()).isEqualTo(0.0)
  }

  @Test
  fun when_new_blobs_exceed_time_deadline_verify_trigger_type_and_aggregation() {
    val aggregationDeadline = 100.milliseconds
    val aggregationDeadlineDelay = 50.milliseconds

    var aggregation: BlobsToAggregate? = null
    val globalAggregationCalculator =
      aggregationCalculator(
        proofLimit = 1500u,
        aggregationDeadline = aggregationDeadline,
        aggregationDeadlineDelay = aggregationDeadlineDelay,
      ) { blobsToAggregate ->
        aggregation = blobsToAggregate
        SafeFuture.completedFuture(Unit)
      }

    val blob1 = blobCounters(
      numberOfBatches = 5u,
      startBlockNumber = 1u,
      endBlockNumber = 10u,
      startBlockTimestamp = Instant.fromEpochMilliseconds(100),
      endBlockTimestamp = Instant.fromEpochMilliseconds(130),
    )

    val blob2 = blobCounters(
      numberOfBatches = 12u,
      startBlockNumber = blob1.endBlockNumber + 1uL,
      endBlockNumber = 30u,
      startBlockTimestamp = Instant.fromEpochMilliseconds(140),
      endBlockTimestamp = Instant.fromEpochMilliseconds(250),
    )

    whenever(safeBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = blob2.endBlockNumber,
            hash = ByteArrayExt.random32(),
            timestamp = blob2.endBlockTimestamp,
          ),
        ),
      )

    val time1 = blob1.startBlockTimestamp.plus(aggregationDeadline).minus(1.milliseconds)
    // Deadline not exceeded
    fixedClock.setTimeTo(time1)
    globalAggregationCalculator.newBlob(blob1)
    aggregationTriggerCalculatorByDeadline.checkAggregation().get()
    assertThat(aggregation).isNull()

    val time2 = blob2.endBlockTimestamp.plus(aggregationDeadline).plus(aggregationDeadlineDelay).plus(1.milliseconds)
    // Deadline exceeded
    fixedClock.setTimeTo(time2)
    globalAggregationCalculator.newBlob(blob2)
    aggregationTriggerCalculatorByDeadline.checkAggregation().get()

    assertThat(aggregation).isEqualTo(BlobsToAggregate(blob1.startBlockNumber, blob2.endBlockNumber))
    verify(safeBlockProvider, times(1)).getLatestSafeBlockHeader()
  }

  @Test
  fun `metrics are exported correctly when aggregation is triggered by proof limit`() {
    val testMeterRegistry = SimpleMeterRegistry()
    val globalAggregationCalculator = aggregationCalculator(
      proofLimit = 15u,
      metricsFacade = MicrometerMetricsFacade(testMeterRegistry, "test"),
    )
    val pendingProofsGauge = testMeterRegistry.get("test.aggregation.proofs.ready").gauge()
    assertThat(pendingProofsGauge.value()).isEqualTo(0.0)
    globalAggregationCalculator.onAggregation {
      SafeFuture.completedFuture(Unit)
    }

    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 5u,
        startBlockNumber = 1u,
        endBlockNumber = 10u,
      ),
    )
    assertThat(pendingProofsGauge.value()).isEqualTo(6.0)

    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 7u,
        startBlockNumber = 11u,
        endBlockNumber = 30u,
      ),
    )
    assertThat(pendingProofsGauge.value()).isEqualTo(14.0)

    // Next blob should cause aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 9u,
        startBlockNumber = 31u,
        endBlockNumber = 45u,
      ),
    )
    assertThat(pendingProofsGauge.value()).isEqualTo(10.0)

    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 4u,
        startBlockNumber = 46u,
        endBlockNumber = 61u,
      ),
    )
    assertThat(pendingProofsGauge.value()).isEqualTo(0.0)
  }

  @Test
  fun `metrics are exported correctly when aggregation is triggered by deadline`() {
    val testMeterRegistry = SimpleMeterRegistry()
    val globalAggregationCalculator = aggregationCalculator(
      aggregationDeadline = 100.milliseconds,
      aggregationDeadlineDelay = 50.milliseconds,
      metricsFacade = MicrometerMetricsFacade(testMeterRegistry, "test"),
    )

    val pendingProofsGauge = testMeterRegistry.get("test.aggregation.proofs.ready").gauge()
    assertThat(pendingProofsGauge.value()).isEqualTo(0.0)

    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 5u,
        startBlockNumber = 1u,
        endBlockNumber = 10u,
        startBlockTimestamp = Instant.fromEpochMilliseconds(100),
        endBlockTimestamp = Instant.fromEpochMilliseconds(130),
      ),
    )
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 12u,
        startBlockNumber = 11u,
        endBlockNumber = 30u,
        startBlockTimestamp = Instant.fromEpochMilliseconds(140),
        endBlockTimestamp = Instant.fromEpochMilliseconds(250),
      ),
    )

    whenever(safeBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = 30u,
            hash = ByteArrayExt.random32(),
            timestamp = Instant.fromEpochMilliseconds(250),
          ),
        ),
      )
    fixedClock.setTimeTo(Instant.fromEpochMilliseconds(1755))

    assertThat(pendingProofsGauge.value()).isEqualTo(19.0)
    aggregationTriggerCalculatorByDeadline.checkAggregation().get()
    assertThat(pendingProofsGauge.value()).isEqualTo(0.0)
  }

  @Test
  fun `verify new blob between trigger by deadline and aggregation completion can trigger aggregation`() {
    val aggregationDeadline = 500.milliseconds
    val aggregationDeadlineDelay = 100.milliseconds

    val lastFinalizedBlockNumber = 10uL

    val firstBlobStartBlockTimeStamp = Instant.fromEpochMilliseconds(100)
    val firstBlobEndBlockTimeStamp = Instant.fromEpochMilliseconds(500)
    val firstBlobStartBlockNumber = lastFinalizedBlockNumber + 1uL
    val firstBlobEndBlockNumber = 15uL

    val secondBlobStartBlockNumber = firstBlobEndBlockNumber.plus(1u)
    val secondBlobEndBlockNumber = secondBlobStartBlockNumber.plus(15uL)
    val secondBlobStartTimestamp = firstBlobEndBlockTimeStamp.plus(20.milliseconds)
    val secondBlobEndTimestamp = secondBlobStartTimestamp.plus(30.milliseconds)

    val aggregations = ConcurrentLinkedQueue<BlobsToAggregate>()
    val aggregationCount = AtomicInteger()
    val blockAggregation1ProcessingUntilComplete = SafeFuture<Unit>()
    val blockAggregation2ProcessingUntilComplete = SafeFuture<Unit>()

    val aggregationHandler = AggregationHandler { blobsToAggregate ->
      when (blobsToAggregate.endBlockNumber) {
        firstBlobEndBlockNumber -> {
          blockAggregation1ProcessingUntilComplete.thenPeek {
            aggregations.add(blobsToAggregate)
            aggregationCount.incrementAndGet()
          }
        }

        secondBlobEndBlockNumber -> {
          blockAggregation2ProcessingUntilComplete.thenPeek {
            aggregations.add(blobsToAggregate)
            aggregationCount.incrementAndGet()
          }
        }

        else -> {
          SafeFuture.failedFuture(IllegalStateException())
        }
      }
    }

    val globalAggregationCalculator = aggregationCalculator(
      lastBlockNumber = lastFinalizedBlockNumber,
      aggregationDeadline = aggregationDeadline,
      aggregationDeadlineDelay = aggregationDeadlineDelay,
      aggregationHandler = aggregationHandler,
    )

    whenever(safeBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = firstBlobEndBlockNumber,
            hash = ByteArrayExt.random32(),
            timestamp = firstBlobEndBlockTimeStamp,
          ),
        ),
      )

    val time1 = firstBlobEndBlockTimeStamp.plus(aggregationDeadline).plus(aggregationDeadlineDelay).plus(2.milliseconds)
    fixedClock.setTimeTo(time1)
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 3u,
        startBlockNumber = firstBlobStartBlockNumber,
        endBlockNumber = firstBlobEndBlockNumber,
        startBlockTimestamp = firstBlobStartBlockTimeStamp,
        endBlockTimestamp = firstBlobEndBlockTimeStamp,
      ),
    )
    val check1 = aggregationTriggerCalculatorByDeadline.checkAggregation()

    whenever(safeBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = secondBlobEndBlockNumber,
            hash = ByteArrayExt.random32(),
            timestamp = secondBlobEndTimestamp,
          ),
        ),
      )

    val time2 = secondBlobEndTimestamp.plus(aggregationDeadline).plus(aggregationDeadlineDelay).plus(2.milliseconds)
    fixedClock.setTimeTo(time2)

    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 10u,
        startBlockNumber = secondBlobStartBlockNumber,
        endBlockNumber = secondBlobEndBlockNumber,
        startBlockTimestamp = secondBlobStartTimestamp,
        endBlockTimestamp = secondBlobEndTimestamp,
      ),
    )
    blockAggregation1ProcessingUntilComplete.complete(Unit)
    check1.get()

    val check2 = aggregationTriggerCalculatorByDeadline.checkAggregation()

    blockAggregation2ProcessingUntilComplete.complete(Unit)
    SafeFuture.allOf(check1, check2).get()
    assertThat(aggregationCount.get()).isEqualTo(2)
    assertThat(aggregations.size).isEqualTo(2)
    assertThat(aggregations.toList().sortedBy { it.startBlockNumber }).containsExactly(
      BlobsToAggregate(startBlockNumber = firstBlobStartBlockNumber, endBlockNumber = firstBlobEndBlockNumber),
      BlobsToAggregate(startBlockNumber = secondBlobStartBlockNumber, endBlockNumber = secondBlobEndBlockNumber),
    )
  }

  @ParameterizedTest(name = "{0}_{1}")
  @MethodSource("aggregationWithDifferentSizeConstraintTestCases")
  fun `test aggregations with different aggregation size constraints`(
    @Suppress("UNUSED_PARAMETER") name: String,
    aggregationSizeMultipleOf: Int,
    blobs: List<BlobCounters>,
    proofsLimit: Int,
    expectedAggregations: List<BlobsToAggregate>,
  ) {
    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(
      proofLimit = proofsLimit.toUInt(),
      aggregationSizeMultipleOf = aggregationSizeMultipleOf,
    ) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }
    blobs.forEach { globalAggregationCalculator.newBlob(it) }
    assertThat(actualAggregations).containsExactlyElementsOf(expectedAggregations)
  }

  @Test
  fun `test aggregations when all reprocessed blobs trigger aggregation`() {
    val aggregationTriggerOnReprocessCalculator = object : SyncAggregationTriggerCalculator {
      private val seenBlobsSet = mutableSetOf<ULong>()
      private var inFlightAggregation: BlobsToAggregate? = null
      override fun checkAggregationTrigger(blobCounters: BlobCounters): AggregationTrigger? {
        return if (seenBlobsSet.contains(blobCounters.startBlockNumber)) {
          AggregationTrigger(
            AggregationTriggerType.TIME_LIMIT,
            inFlightAggregation ?: BlobsToAggregate(blobCounters.startBlockNumber, blobCounters.endBlockNumber),
          )
        } else {
          seenBlobsSet.add(blobCounters.startBlockNumber)
          null
        }
      }
      override fun newBlob(blobCounters: BlobCounters) {
        inFlightAggregation = BlobsToAggregate(
          inFlightAggregation?.startBlockNumber ?: blobCounters.startBlockNumber,
          blobCounters.endBlockNumber,
        )
      }
      override fun reset() {
        inFlightAggregation = null
      }
    }
    val proofsLimit = 15
    val aggregationSizeMultipleOf = 4
    val blobs = regularBlobs(8)
    val expectedAggregations = listOf(
      BlobsToAggregate(1uL, 4uL),
      BlobsToAggregate(5uL, 5uL),
      BlobsToAggregate(6uL, 6uL),
      BlobsToAggregate(7uL, 7uL),
      BlobsToAggregate(8uL, 8uL),
    )
    val aggregationTriggerCalculator = AggregationTriggerCalculatorByProofLimit(proofsLimit.toUInt())
    val globalAggregationCalculator = GlobalAggregationCalculator(
      0u,
      syncAggregationTrigger = listOf(aggregationTriggerCalculator, aggregationTriggerOnReprocessCalculator),
      deferredAggregationTrigger = emptyList(),
      metricsFacade = mock<MetricsFacade>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
      aggregationSizeMultipleOf = aggregationSizeMultipleOf.toUInt(),
    )
    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val aggregationHandler = AggregationHandler { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }
    globalAggregationCalculator.onAggregation(aggregationHandler)
    blobs.forEach { globalAggregationCalculator.newBlob(it) }
    assertThat(actualAggregations).containsExactlyElementsOf(expectedAggregations)
  }

  @Test
  fun `when sync and deferred calculators trigger aggregation simultaneously the one processed later should fail`() {
    val proofsLimit = 10
    val aggregationSizeMultipleOf = 2
    val blobs = regularBlobs(6, 2)
    val deferredAggregationTrigger = AggregationTrigger(
      aggregationTriggerType = AggregationTriggerType.TIME_LIMIT,
      aggregation = BlobsToAggregate(
        startBlockNumber = blobs[0].startBlockNumber,
        endBlockNumber = blobs[2].endBlockNumber,
      ),
    )
    val expectedAggregations = listOf(
      BlobsToAggregate(
        startBlockNumber = blobs[0].startBlockNumber,
        endBlockNumber = blobs[1].endBlockNumber,
      ),
      BlobsToAggregate(
        startBlockNumber = blobs[2].startBlockNumber,
        endBlockNumber = blobs[3].endBlockNumber,
      ),
    )
    val syncAggregationTriggerCalculator = AggregationTriggerCalculatorByProofLimit(proofsLimit.toUInt())
    val deferredAggregationTriggerCalculator = object : DeferredAggregationTriggerCalculator {
      private var aggregationTriggerHandler = AggregationTriggerHandler.NOOP_HANDLER
      override fun onAggregationTrigger(aggregationTriggerHandler: AggregationTriggerHandler) {
        this.aggregationTriggerHandler = aggregationTriggerHandler
      }
      override fun newBlob(blobCounters: BlobCounters) {}
      override fun reset() {}
      fun triggerAggregation() {
        aggregationTriggerHandler.onAggregationTrigger(deferredAggregationTrigger)
      }
    }
    val globalAggregationCalculator = GlobalAggregationCalculator(
      0u,
      syncAggregationTrigger = listOf(syncAggregationTriggerCalculator),
      deferredAggregationTrigger = listOf(deferredAggregationTriggerCalculator),
      metricsFacade = mock<MetricsFacade>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
      aggregationSizeMultipleOf = aggregationSizeMultipleOf.toUInt(),
    )

    val enableDeferredTrigger = AtomicBoolean(false)
    val executorService = Executors.newFixedThreadPool(2)
    val proofAggregationCoordinatorService = Runnable {
      blobs.forEach { globalAggregationCalculator.newBlob(it) }
    }

    val deferredRunner = Runnable {
      while (!enableDeferredTrigger.get()) {
        Thread.sleep(10)
      }
      deferredAggregationTriggerCalculator.triggerAggregation()
    }

    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val aggregationHandler = AggregationHandler { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      enableDeferredTrigger.set(true)
      Thread.sleep(1000)
      SafeFuture.completedFuture(Unit)
    }
    globalAggregationCalculator.onAggregation(aggregationHandler)

    val proofAggregationCoordinatorServiceTask = executorService.submit(proofAggregationCoordinatorService)
    val deferredRunnerResult = executorService.submit(deferredRunner)

    assertDoesNotThrow { proofAggregationCoordinatorServiceTask.get() }
    val exception = assertThrows<ExecutionException> { deferredRunnerResult.get() }
    executorService.shutdown()
    assertThat(exception.cause).isExactlyInstanceOf(IllegalStateException::class.java)
    assertThat(exception.cause?.message)
      .contains("Aggregation triggered when pending blobs do not contain blobs within aggregation interval")
    assertThat(exception.cause?.message).contains("aggregationTriggerType=TIME_LIMIT")
    assertThat(actualAggregations).containsExactlyElementsOf(expectedAggregations)
  }

  @Test
  fun `test getUpdatedAggregationSize`() {
    checkAggregationSizesNotExceedingMaxAggregationSize(12u, 1u)
    assertThat(GlobalAggregationCalculator.getUpdatedAggregationSize(11u, 1u)).isEqualTo(11u)
    assertThat(GlobalAggregationCalculator.getUpdatedAggregationSize(12u, 1u)).isEqualTo(12u)

    checkAggregationSizesNotExceedingMaxAggregationSize(6u, 6u)
    assertThat(GlobalAggregationCalculator.getUpdatedAggregationSize(11u, 6u)).isEqualTo(6u)
    assertThat(GlobalAggregationCalculator.getUpdatedAggregationSize(12u, 6u)).isEqualTo(12u)

    checkAggregationSizesNotExceedingMaxAggregationSize(9u, 9u)
    assertThat(GlobalAggregationCalculator.getUpdatedAggregationSize(11u, 9u)).isEqualTo(9u)
    assertThat(GlobalAggregationCalculator.getUpdatedAggregationSize(12u, 9u)).isEqualTo(9u)
    assertThat(GlobalAggregationCalculator.getUpdatedAggregationSize(18u, 9u)).isEqualTo(18u)
  }

  @Test
  fun `when blob timestamp crosses hard fork boundary then trigger aggregation`() {
    val initialTimestamp = Instant.fromEpochMilliseconds(1000)
    val hardForkTimestamp = Instant.fromEpochMilliseconds(2000)
    val beforeHardFork = Instant.fromEpochMilliseconds(1900)
    val afterHardFork = Instant.fromEpochMilliseconds(2100)

    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(
      proofLimit = 1500u,
      hardForkTimestamps = listOf(hardForkTimestamp),
      initialTimestamp = initialTimestamp,
    ) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // First blob before hard fork - should not trigger aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 5u,
        startBlockNumber = 1u,
        endBlockNumber = 10u,
        startBlockTimestamp = beforeHardFork.minus(100.milliseconds),
        endBlockTimestamp = beforeHardFork.minus(50.milliseconds), // Ends before hard fork
      ),
    )
    assertThat(actualAggregations).isEmpty()

    // Second blob that starts after hard fork - should trigger aggregation of previous blob
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 11u,
        endBlockNumber = 20u,
        startBlockTimestamp = afterHardFork.plus(50.milliseconds), // Starts after hard fork
        endBlockTimestamp = afterHardFork.plus(100.milliseconds), // Ends after hard fork
      ),
    )

    assertThat(actualAggregations).hasSize(1)
    assertThat(actualAggregations[0]).isEqualTo(BlobsToAggregate(1u, 10u))
  }

  @Test
  fun `when blob timestamp equals hard fork timestamp then trigger aggregation`() {
    val initialTimestamp = Instant.fromEpochMilliseconds(1000)
    val hardForkTimestamp = Instant.fromEpochMilliseconds(2000)
    val beforeHardFork = Instant.fromEpochMilliseconds(1900)
    val afterHardFork = Instant.fromEpochMilliseconds(2100)

    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(
      proofLimit = 1500u,
      hardForkTimestamps = listOf(hardForkTimestamp),
      initialTimestamp = initialTimestamp,
    ) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // First blob before hard fork - ends before hard fork
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 5u,
        startBlockNumber = 1u,
        endBlockNumber = 10u,
        startBlockTimestamp = beforeHardFork.minus(100.milliseconds),
        endBlockTimestamp = beforeHardFork.minus(50.milliseconds), // Ends before hard fork
      ),
    )

    // Second blob that starts exactly at hard fork timestamp - this is valid
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 11u,
        endBlockNumber = 20u,
        startBlockTimestamp = hardForkTimestamp, // Starts exactly at hard fork (first block of new fork)
        endBlockTimestamp = afterHardFork, // Ends after hard fork
      ),
    )

    // Should trigger aggregation of the first blob (which ended before hard fork)
    assertThat(actualAggregations).hasSize(1)
    assertThat(actualAggregations[0]).isEqualTo(BlobsToAggregate(1u, 10u))
  }

  @Test
  fun `when multiple hard fork timestamps then handle multiple triggers correctly`() {
    val initialTimestamp = Instant.fromEpochMilliseconds(1000)
    val firstHardFork = Instant.fromEpochMilliseconds(2000)
    val secondHardFork = Instant.fromEpochMilliseconds(3000)

    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(
      proofLimit = 1500u,
      hardForkTimestamps = listOf(firstHardFork, secondHardFork),
      initialTimestamp = initialTimestamp,
    ) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // First blob before first hard fork
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 5u,
        startBlockNumber = 1u,
        endBlockNumber = 10u,
        startBlockTimestamp = Instant.fromEpochMilliseconds(1500),
        endBlockTimestamp = Instant.fromEpochMilliseconds(1900), // Ends before first hard fork
      ),
    )

    // Second blob after first hard fork - should trigger aggregation of first blob
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 11u,
        endBlockNumber = 20u,
        startBlockTimestamp = Instant.fromEpochMilliseconds(2100), // Starts after first hard fork
        endBlockTimestamp = Instant.fromEpochMilliseconds(2200),
      ),
    )

    // Should trigger first aggregation
    assertThat(actualAggregations).hasSize(1)
    assertThat(actualAggregations[0]).isEqualTo(BlobsToAggregate(1u, 10u))

    // Third blob between hard forks
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 4u,
        startBlockNumber = 21u,
        endBlockNumber = 30u,
        startBlockTimestamp = Instant.fromEpochMilliseconds(2300),
        endBlockTimestamp = Instant.fromEpochMilliseconds(2800), // Ends before second hard fork
      ),
    )
    // Still doesn't trigger the aggregation
    assertThat(actualAggregations).hasSize(1)

    // Fourth blob after second hard fork - should trigger aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 2u,
        startBlockNumber = 31u,
        endBlockNumber = 40u,
        startBlockTimestamp = Instant.fromEpochMilliseconds(3100), // Starts after second hard fork
        endBlockTimestamp = Instant.fromEpochMilliseconds(3200),
      ),
    )

    // Should trigger second aggregation
    assertThat(actualAggregations).hasSize(2)
    assertThat(actualAggregations[1]).isEqualTo(BlobsToAggregate(11u, 30u))
  }

  @Test
  fun `when hard fork trigger combines with proof limit trigger then handle correctly`() {
    val initialTimestamp = Instant.fromEpochMilliseconds(1000)
    val hardForkTimestamp = Instant.fromEpochMilliseconds(2000)
    val beforeHardFork = Instant.fromEpochMilliseconds(1900)
    val afterHardFork = Instant.fromEpochMilliseconds(2100)

    val actualAggregations = mutableListOf<BlobsToAggregate>()
    val globalAggregationCalculator = aggregationCalculator(
      proofLimit = 10u, // Low proof limit to trigger aggregation
      hardForkTimestamps = listOf(hardForkTimestamp),
      initialTimestamp = initialTimestamp,
    ) { blobsToAggregate ->
      actualAggregations.add(blobsToAggregate)
      SafeFuture.completedFuture(Unit)
    }

    // First blob with high proof count - should trigger proof limit aggregation
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 9u, // 10 proofs total
        startBlockNumber = 1u,
        endBlockNumber = 10u,
        startBlockTimestamp = beforeHardFork.minus(200.milliseconds),
        endBlockTimestamp = beforeHardFork.minus(100.milliseconds),
      ),
    )

    // Should trigger aggregation due to proof limit
    assertThat(actualAggregations).hasSize(1)
    assertThat(actualAggregations[0]).isEqualTo(BlobsToAggregate(1u, 10u))

    // Second blob before hard fork
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 11u,
        endBlockNumber = 20u,
        startBlockTimestamp = beforeHardFork.minus(50.milliseconds),
        endBlockTimestamp = beforeHardFork.minus(10.milliseconds), // Ends before hard fork
      ),
    )

    // Third blob that starts after hard fork - should trigger aggregation due to hard fork
    globalAggregationCalculator.newBlob(
      blobCounters(
        numberOfBatches = 2u,
        startBlockNumber = 21u,
        endBlockNumber = 30u,
        startBlockTimestamp = afterHardFork.plus(50.milliseconds), // Starts after hard fork
        endBlockTimestamp = afterHardFork.plus(100.milliseconds),
      ),
    )

    // Should trigger second aggregation due to hard fork
    assertThat(actualAggregations).hasSize(2)
    assertThat(actualAggregations[1]).isEqualTo(BlobsToAggregate(11u, 20u))
  }

  companion object {
    data class AggregationSizeConstraintTestCase(
      val name: String,
      val aggregationSizeMultipleOf: Int,
      val blobs: List<BlobCounters>,
      val proofsLimit: Int,
      val expectedAggregations: List<BlobsToAggregate>,
    )

    private fun checkAggregationSizesNotExceedingMaxAggregationSize(endSize: UInt, maxAggregationSize: UInt) {
      for (aggregationSize in 1u..endSize) {
        assertThat(
          GlobalAggregationCalculator.getUpdatedAggregationSize(aggregationSize, maxAggregationSize),
        ).isEqualTo(aggregationSize)
      }
    }

    private fun regularBlobs(count: Int, batchSize: Int = 1): MutableList<BlobCounters> {
      return (1..count).map { i -> blob(i, i, batchSize) }.toMutableList()
    }

    private fun blob(startBlockNumber: Int, endBlockNumber: Int, numberOfBatches: Int): BlobCounters {
      return BlobCounters(
        numberOfBatches = numberOfBatches.toUInt(),
        startBlockNumber = startBlockNumber.toULong(),
        endBlockNumber = endBlockNumber.toULong(),
        startBlockTimestamp = Instant.fromEpochMilliseconds((startBlockNumber * 100).toLong()),
        endBlockTimestamp = Instant.fromEpochMilliseconds((endBlockNumber * 100).toLong()),
        expectedShnarf = Random.nextBytes(32),
      )
    }

    private val aggregationSizeConstraintTestCase = listOf<AggregationSizeConstraintTestCase>(
      AggregationSizeConstraintTestCase(
        name = "regular_blobs_with_customization",
        aggregationSizeMultipleOf = 1,
        blobs = regularBlobs(15) + mutableListOf(blob(16, 20, 14)),
        proofsLimit = 15,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 7u),
          BlobsToAggregate(8u, 14u),
          BlobsToAggregate(15u, 15u),
          BlobsToAggregate(16u, 20u),
        ),
      ),
      AggregationSizeConstraintTestCase(
        name = "regular_blobs_with_customization",
        aggregationSizeMultipleOf = 2,
        blobs = regularBlobs(15) + mutableListOf(blob(16, 20, 10), blob(21, 25, 14)),
        proofsLimit = 15,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 6u),
          BlobsToAggregate(7u, 12u),
          BlobsToAggregate(13u, 14u),
          BlobsToAggregate(15u, 20u),
          BlobsToAggregate(21u, 25u),
        ),
      ),
      AggregationSizeConstraintTestCase(
        name = "regular_blobs",
        aggregationSizeMultipleOf = 3,
        blobs = regularBlobs(15),
        proofsLimit = 15,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 6u),
          BlobsToAggregate(7u, 12u),
        ),
      ),
      AggregationSizeConstraintTestCase(
        name = "regular_blobs",
        aggregationSizeMultipleOf = 4,
        blobs = regularBlobs(15),
        proofsLimit = 15,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 4u),
          BlobsToAggregate(5u, 8u),
        ),
      ),
      AggregationSizeConstraintTestCase(
        name = "regular_blobs",
        aggregationSizeMultipleOf = 5,
        blobs = regularBlobs(15),
        proofsLimit = 21,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 10u),
        ),
      ),
      AggregationSizeConstraintTestCase(
        name = "regular_blobs",
        aggregationSizeMultipleOf = 6,
        blobs = regularBlobs(30),
        proofsLimit = 26,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 12u),
          BlobsToAggregate(13u, 24u),
        ),
      ),
      AggregationSizeConstraintTestCase(
        name = "regular_blobs",
        aggregationSizeMultipleOf = 7,
        blobs = regularBlobs(30),
        proofsLimit = 26,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 7u),
          BlobsToAggregate(8u, 14u),
          BlobsToAggregate(15u, 21u),
        ),
      ),
      AggregationSizeConstraintTestCase(
        name = "regular_blobs",
        aggregationSizeMultipleOf = 8,
        blobs = regularBlobs(30),
        proofsLimit = 26,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 8u),
          BlobsToAggregate(9u, 16u),
          BlobsToAggregate(17u, 24u),
        ),
      ),
      AggregationSizeConstraintTestCase(
        name = "regular_blobs",
        aggregationSizeMultipleOf = 9,
        blobs = regularBlobs(30),
        proofsLimit = 26,
        expectedAggregations = listOf(
          BlobsToAggregate(1u, 9u),
          BlobsToAggregate(10u, 18u),
        ),
      ),
    )

    @JvmStatic
    fun aggregationWithDifferentSizeConstraintTestCases(): Stream<Arguments> {
      return aggregationSizeConstraintTestCase.map {
        Arguments.of(
          it.name,
          it.aggregationSizeMultipleOf,
          it.blobs,
          it.proofsLimit,
          it.expectedAggregations,
        )
      }.stream()
    }
  }
}
