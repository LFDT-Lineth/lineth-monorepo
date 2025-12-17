package net.consensys.zkevm.ethereum.coordination.aggregation

import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.domain.BlockHeaderSummary
import linea.kotlin.ByteArrayExt
import net.consensys.FakeFixedClock
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random
import kotlin.time.Duration.Companion.days
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes

class AggregationTriggerCalculatorByDeadlineTest {
  @Test
  fun `trigger aggregation when past deadline with activity on l2`() {
    val fakeClock = FakeFixedClock(Instant.parse("2025-01-01T00:00:00.000Z"))
    val mockLatestSafeBlockProvider = mock<SafeBlockProvider>()
    val aggregationDeadline = 10.days

    var aggregationTrigger: AggregationTriggerType? = null
    val aggregationTriggerTypeHandler =
      AggregationTriggerHandler {
        aggregationTrigger = it.aggregationTriggerType
        SafeFuture.completedFuture(Unit)
      }

    val aggregationTriggerByDeadline =
      AggregationTriggerCalculatorByDeadline(
        AggregationTriggerCalculatorByDeadline.Config(
          aggregationDeadline,
          noL2ActivityTimeout = 5.minutes,
          waitForNoL2ActivityToTriggerAggregation = false,
        ),
        fakeClock,
        mockLatestSafeBlockProvider,
      ).apply {
        onAggregationTrigger(aggregationTriggerTypeHandler)
      }

    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(aggregationTrigger).isNull()

    aggregationTriggerByDeadline.newBlob(
      BlobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 100UL,
        endBlockNumber = 110UL,
        startBlockTimestamp = fakeClock.now().minus(5.days),
        endBlockTimestamp = fakeClock.now().minus(3.days),
        expectedShnarf = Random.nextBytes(32),
      ),
    )
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(aggregationTrigger).isNull()

    aggregationTriggerByDeadline.newBlob(
      BlobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 111UL,
        endBlockNumber = 120UL,
        startBlockTimestamp = fakeClock.now().minus(3.days),
        endBlockTimestamp = fakeClock.now().minus(1.days),
        expectedShnarf = Random.nextBytes(32),
      ),
    )
    fakeClock.advanceBy(10.days)
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(aggregationTrigger).isEqualTo(AggregationTriggerType.TIME_LIMIT)
  }

  @Test
  fun `trigger aggregation when past deadline, latest block match and no activity on l2`() {
    val mockClock = mock<Clock>()
    val mockLatestSafeBlockProvider = mock<SafeBlockProvider>()

    val aggregationDeadline = 200.milliseconds
    val aggregationDeadlineDelay = 100.milliseconds
    val latestBlockTimestamp = Instant.fromEpochMilliseconds(500)
    val blobStartBlockTimeStamp = Instant.fromEpochMilliseconds(350)
    val blobEndBlockTimeStamp = Instant.fromEpochMilliseconds(490)

    val latestBlockNumber = 16uL
    val blobStartBlockNumber = 10uL
    val blobEndBlockNumber = latestBlockNumber
    whenever(mockLatestSafeBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = latestBlockNumber,
            hash = ByteArrayExt.random32(),
            timestamp = latestBlockTimestamp,
          ),
        ),
      )

    val deadlineTriggered = SafeFuture<AggregationTrigger?>()
    val aggregationTriggerTypeHandler =
      AggregationTriggerHandler {
        if (it.aggregationTriggerType == AggregationTriggerType.TIME_LIMIT) {
          deadlineTriggered.complete(it)
        } else {
          deadlineTriggered.complete(null)
        }
        SafeFuture.completedFuture(Unit)
      }

    val aggregationTriggerByDeadline =
      AggregationTriggerCalculatorByDeadline(
        AggregationTriggerCalculatorByDeadline.Config(aggregationDeadline, aggregationDeadlineDelay, true),
        mockClock,
        mockLatestSafeBlockProvider,
      )

    aggregationTriggerByDeadline.onAggregationTrigger(aggregationTriggerTypeHandler)

    whenever(mockClock.now()).thenReturn(Instant.fromEpochMilliseconds(50))
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isNotCompleted

    aggregationTriggerByDeadline.newBlob(
      BlobCounters(
        numberOfBatches = 3u,
        startBlockNumber = blobStartBlockNumber,
        endBlockNumber = blobEndBlockNumber,
        startBlockTimestamp = blobStartBlockTimeStamp,
        endBlockTimestamp = blobEndBlockTimeStamp,
        expectedShnarf = Random.nextBytes(32),
      ),
    )

    val time1 = blobEndBlockTimeStamp.plus(1.milliseconds) // 491
    whenever(mockClock.now()).thenReturn(time1)
    aggregationTriggerByDeadline.checkAggregation().get()
    // 491 -  aggregationDeadline(200) = 291, 291 < blobStartBlockTimeStamp(350) -> Deadline not exceeded
    // No aggregation trigger
    assertThat(deadlineTriggered).isNotCompleted

    val time2 = blobStartBlockTimeStamp.plus(aggregationDeadline).plus(1.milliseconds) // 551
    whenever(mockClock.now()).thenReturn(time2)
    aggregationTriggerByDeadline.checkAggregation().get()
    // 551 - aggregationDeadline(200) = 351, 351 > blobStartBlockTimeStamp(350) -> Deadline exceeded
    // 551 - aggregationDeadlineDelay(100) = 451, 451 <  latestBlockTimestamp(500) -> noActivityOnL2 = false
    // blobEndBlockNumber(16) == latestBlockNumber (16) && noActivityOnL2 -> true && false = false
    // No aggregation trigger
    assertThat(deadlineTriggered).isNotCompleted

    val time3 = latestBlockTimestamp.plus(aggregationDeadlineDelay).plus(1.milliseconds) // 601
    whenever(mockClock.now()).thenReturn(time3)
    // 601 - aggregationDeadline(200) = 401, 401 > blobStartBlockTimeStamp(350) -> Deadline exceeded
    // 501 - aggregationDeadlineDelay(100) = 501, 501 >  latestBlockTimestamp(500) -> noActivityOnL2 = true
    // blobEndBlockNumber(16) == latestBlockNumber (16) && noActivityOnL2 -> true && true = true
    // Aggregation trigger
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isCompleted
    val aggregationTrigger = deadlineTriggered.get()
    assertThat(aggregationTrigger).isNotNull
    assertThat(aggregationTrigger!!.aggregationTriggerType).isEqualTo(AggregationTriggerType.TIME_LIMIT)
    assertThat(aggregationTrigger.aggregation).isEqualTo(BlobsToAggregate(blobStartBlockNumber, blobEndBlockNumber))
  }

  @Test
  fun when_reset_verify_deadline_state_is_reset() {
    val mockClock = mock<Clock>()
    val mockLatestSafeBlockProvider = mock<SafeBlockProvider>()

    val deadlineTriggered = SafeFuture<AggregationTrigger?>()
    val aggregationTriggerTypeHandler =
      AggregationTriggerHandler {
        if (it.aggregationTriggerType == AggregationTriggerType.TIME_LIMIT) {
          deadlineTriggered.complete(it)
        } else {
          deadlineTriggered.complete(null)
        }
        SafeFuture.completedFuture(Unit)
      }

    val deadline = 200.milliseconds
    val aggregationDeadlineDelay = 100.milliseconds

    val aggregationTriggerByDeadline =
      AggregationTriggerCalculatorByDeadline(
        AggregationTriggerCalculatorByDeadline.Config(deadline, aggregationDeadlineDelay, true),
        mockClock,
        mockLatestSafeBlockProvider,
      )
    aggregationTriggerByDeadline.onAggregationTrigger(aggregationTriggerTypeHandler)

    whenever(mockClock.now()).thenReturn(Instant.fromEpochMilliseconds(50))
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isNotCompleted

    val firstBloblStartBlockTimeStamp = Instant.fromEpochMilliseconds(100)
    val firstBlobEndBlockTimeStamp = firstBloblStartBlockTimeStamp.plus(50.milliseconds)

    aggregationTriggerByDeadline.newBlob(
      BlobCounters(
        numberOfBatches = 3u,
        startBlockNumber = 11u,
        endBlockNumber = 15u,
        startBlockTimestamp = firstBloblStartBlockTimeStamp,
        endBlockTimestamp = firstBlobEndBlockTimeStamp,
        expectedShnarf = Random.nextBytes(32),
      ),
    )

    whenever(mockClock.now()).thenReturn(
      firstBlobEndBlockTimeStamp
        .plus(1.milliseconds),
    )
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isNotCompleted

    // Reset the deadline trigger calculator
    aggregationTriggerByDeadline.reset()

    whenever(mockClock.now())
      .thenReturn(firstBloblStartBlockTimeStamp.plus(deadline).plus(1.milliseconds))
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isNotCompleted

    val secondBlobStartBlockTimestamp = Instant.fromEpochMilliseconds(500)
    val secondBlobEndBlockTimestamp = secondBlobStartBlockTimestamp.plus(50.milliseconds)

    val blobStartBlockNumber = 11uL
    val blobEndBlockNumber = 15uL

    aggregationTriggerByDeadline.newBlob(
      BlobCounters(
        numberOfBatches = 3u,
        startBlockNumber = blobStartBlockNumber,
        endBlockNumber = blobEndBlockNumber,
        startBlockTimestamp = secondBlobStartBlockTimestamp,
        endBlockTimestamp = secondBlobEndBlockTimestamp,
        expectedShnarf = Random.nextBytes(32),
      ),
    )
    whenever(mockLatestSafeBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = 15u,
            hash = ByteArrayExt.random32(),
            timestamp = firstBlobEndBlockTimeStamp,
          ),
        ),
      )
    whenever(mockClock.now()).thenReturn(secondBlobStartBlockTimestamp.plus(deadline).plus(1.milliseconds))
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isCompleted
    val aggregationTrigger = deadlineTriggered.get()
    assertThat(aggregationTrigger).isNotNull
    assertThat(aggregationTrigger!!.aggregationTriggerType).isEqualTo(AggregationTriggerType.TIME_LIMIT)
    assertThat(aggregationTrigger.aggregation).isEqualTo(BlobsToAggregate(blobStartBlockNumber, blobEndBlockNumber))
  }

  @Test
  fun `no trigger when latest block does not match aggregation end block`() {
    val mockClock = mock<Clock>()
    val mockLatestSafeBlockProvider = mock<SafeBlockProvider>()

    val aggregationDeadline = 200.milliseconds
    val aggregationDeadlineDelay = 100.milliseconds
    val latestBlockTimestamp = Instant.fromEpochMilliseconds(500)
    val blobStartBlockTimeStamp = Instant.fromEpochMilliseconds(350)
    val blobEndBlockTimeStamp = Instant.fromEpochMilliseconds(490)

    val latestBlockNumber = 16uL
    val blobStartBlockNumber = 10uL
    val blobEndBlockNumber = latestBlockNumber - 1uL
    whenever(mockLatestSafeBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = latestBlockNumber,
            hash = ByteArrayExt.random32(),
            timestamp = latestBlockTimestamp,
          ),
        ),
      )

    val deadlineTriggered = SafeFuture<AggregationTrigger?>()
    val aggregationTriggerTypeHandler =
      AggregationTriggerHandler {
        if (it.aggregationTriggerType == AggregationTriggerType.TIME_LIMIT) {
          deadlineTriggered.complete(it)
        } else {
          deadlineTriggered.complete(null)
        }
        SafeFuture.completedFuture(Unit)
      }

    val aggregationTriggerByDeadline =
      AggregationTriggerCalculatorByDeadline(
        AggregationTriggerCalculatorByDeadline.Config(aggregationDeadline, aggregationDeadlineDelay, true),
        mockClock,
        mockLatestSafeBlockProvider,
      )

    aggregationTriggerByDeadline.onAggregationTrigger(aggregationTriggerTypeHandler)

    whenever(mockClock.now()).thenReturn(Instant.fromEpochMilliseconds(50))
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isNotCompleted

    aggregationTriggerByDeadline.newBlob(
      BlobCounters(
        numberOfBatches = 3u,
        startBlockNumber = blobStartBlockNumber,
        endBlockNumber = blobEndBlockNumber,
        startBlockTimestamp = blobStartBlockTimeStamp,
        endBlockTimestamp = blobEndBlockTimeStamp,
        expectedShnarf = Random.nextBytes(32),
      ),
    )

    val time1 = blobEndBlockTimeStamp.plus(1.milliseconds) // 491
    whenever(mockClock.now()).thenReturn(time1)
    aggregationTriggerByDeadline.checkAggregation().get()
    // 491 -  aggregationDeadline(200) = 291, 291 < blobStartBlockTimeStamp(350)
    // Deadline not exceeded
    // No aggregation trigger
    assertThat(deadlineTriggered).isNotCompleted

    val time2 = blobStartBlockTimeStamp.plus(aggregationDeadline).plus(1.milliseconds) // 551
    whenever(mockClock.now()).thenReturn(time2)
    aggregationTriggerByDeadline.checkAggregation().get()
    // 551 - aggregationDeadline(200) = 351, 351 > blobStartBlockTimeStamp(350)
    // Deadline exceeded
    // 551 - aggregationDeadlineDelay(100) = 451, 451 <  latestBlockTimestamp(500)
    // noActivityOnL2 = false
    // blobEndBlockNumber(15) == latestBlockNumber (16) && noActivityOnL2 -> false && false = false
    // No aggregation trigger
    assertThat(deadlineTriggered).isNotCompleted

    val time3 = latestBlockTimestamp.plus(aggregationDeadlineDelay).plus(1.milliseconds) // 601
    whenever(mockClock.now()).thenReturn(time3)
    // 601 - aggregationDeadline(200) = 401, 401 > blobStartBlockTimeStamp(350)
    // Deadline exceeded
    // 601 - aggregationDeadlineDelay(100) = 501, 501 >  latestBlockTimestamp(500)
    // noActivityOnL2 = true
    // blobEndBlockNumber(15) == latestBlockNumber (16) && noActivityOnL2 -> false && true = false
    // No aggregation trigger
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isNotCompleted

    aggregationTriggerByDeadline.newBlob(
      BlobCounters(
        numberOfBatches = 3u,
        startBlockNumber = latestBlockNumber,
        endBlockNumber = latestBlockNumber,
        startBlockTimestamp = latestBlockTimestamp,
        endBlockTimestamp = latestBlockTimestamp,
        expectedShnarf = Random.nextBytes(32),
      ),
    )
    val time4 = time3.plus(10.milliseconds) // 611
    whenever(mockClock.now()).thenReturn(time4)
    // 611 - aggregationDeadline(200) = 411, 411 > blobStartBlockTimeStamp(350)
    // Deadline exceeded
    // 611 - aggregationDeadlineDelay(100) = 511, 511 >  latestBlockTimestamp(500)
    // noActivityOnL2 = true
    // blobEndBlockNumber(16) == latestBlockNumber (16) && noActivityOnL2 -> true && true = true
    // No aggregation trigger
    aggregationTriggerByDeadline.checkAggregation().get()
    assertThat(deadlineTriggered).isCompleted
    val aggregationTrigger = deadlineTriggered.get()
    assertThat(aggregationTrigger).isNotNull
    assertThat(aggregationTrigger!!.aggregationTriggerType).isEqualTo(AggregationTriggerType.TIME_LIMIT)
    assertThat(aggregationTrigger.aggregation).isEqualTo(BlobsToAggregate(blobStartBlockNumber, latestBlockNumber))
  }
}
