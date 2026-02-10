package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.BlobsToAggregate
import net.consensys.zkevm.domain.blobCounters
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class AggregationTriggerCalculatorByTimestampHardForkTest {
  private val baseTimestamp = Instant.parse("2023-01-01T10:00:00Z")
  private val hardForkTimestamp = Instant.parse("2023-01-01T12:00:00Z")
  private val beforeHardFork = Instant.parse("2023-01-01T11:59:59Z")
  private val afterHardFork = Instant.parse("2023-01-01T12:00:01Z")

  @Test
  fun `should require non-empty timestamps in constructor`() {
    assertThatThrownBy {
      AggregationTriggerCalculatorByTimestampHardFork(emptyList(), baseTimestamp)
    }.isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `should require sorted timestamps in constructor`() {
    val unsortedTimestamps = listOf(
      Instant.parse("2023-01-01T12:00:00Z"),
      Instant.parse("2023-01-01T10:00:00Z"), // Earlier timestamp after later one
      Instant.parse("2023-01-01T14:00:00Z"),
    )

    assertThatThrownBy {
      AggregationTriggerCalculatorByTimestampHardFork(unsortedTimestamps, baseTimestamp)
    }.isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `should accept sorted timestamps in constructor`() {
    val sortedTimestamps = listOf(
      Instant.parse("2023-01-01T10:00:00Z"),
      Instant.parse("2023-01-01T12:00:00Z"),
      Instant.parse("2023-01-01T14:00:00Z"),
    )

    // Should not throw
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(sortedTimestamps, baseTimestamp)
    assertThat(calculator).isNotNull
  }

  @Test
  fun `should not trigger when no blob has been processed`() {
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(listOf(hardForkTimestamp), baseTimestamp)
    val blobCounters = blobCounters(startBlockNumber = 1UL, endBlockNumber = 10UL, endBlockTimestamp = afterHardFork)

    val result = calculator.checkAggregationTrigger(blobCounters)

    assertThat(result).isNull()
  }

  @Test
  fun `should not trigger when blob timestamp is before hard fork`() {
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(listOf(hardForkTimestamp), baseTimestamp)
    val blobCounters = blobCounters(startBlockNumber = 1UL, endBlockNumber = 10UL, endBlockTimestamp = beforeHardFork)

    calculator.newBlob(blobCounters)
    val result = calculator.checkAggregationTrigger(blobCounters)

    assertThat(result).isNull()
  }

  @Test
  fun `should trigger when blob timestamp crosses hard fork boundary`() {
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(listOf(hardForkTimestamp), baseTimestamp)
    val firstBlob = blobCounters(startBlockNumber = 1UL, endBlockNumber = 10UL, endBlockTimestamp = beforeHardFork)
    val secondBlob = blobCounters(startBlockNumber = 1UL, endBlockNumber = 20UL, endBlockTimestamp = afterHardFork)

    calculator.newBlob(firstBlob)
    val result = calculator.checkAggregationTrigger(secondBlob)

    assertThat(result).isEqualTo(
      AggregationTrigger(
        aggregationTriggerType = AggregationTriggerType.HARD_FORK,
        aggregation = BlobsToAggregate(1UL, 10UL),
      ),
    )
  }

  @Test
  fun `should trigger when blob timestamp equals hard fork timestamp`() {
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(listOf(hardForkTimestamp), baseTimestamp)
    val firstBlob = blobCounters(startBlockNumber = 1UL, endBlockNumber = 10UL, endBlockTimestamp = beforeHardFork)
    val secondBlob = blobCounters(startBlockNumber = 1UL, endBlockNumber = 20UL, endBlockTimestamp = hardForkTimestamp)

    calculator.newBlob(firstBlob)
    val result = calculator.checkAggregationTrigger(secondBlob)

    assertThat(result).isEqualTo(
      AggregationTrigger(
        aggregationTriggerType = AggregationTriggerType.HARD_FORK,
        aggregation = BlobsToAggregate(1UL, 10UL),
      ),
    )
  }

  @Test
  fun `should handle multiple hard fork timestamps`() {
    val firstFork = Instant.parse("2023-01-01T12:00:00Z")
    val secondFork = Instant.parse("2023-01-01T18:00:00Z")
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(
      listOf(firstFork, secondFork),
      baseTimestamp,
    )

    val firstBlob =
      blobCounters(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        endBlockTimestamp = Instant.parse("2023-01-01T11:00:00Z"),
      )
    val secondBlob =
      blobCounters(
        startBlockNumber = 11UL,
        endBlockNumber = 20UL,
        endBlockTimestamp = Instant.parse("2023-01-01T13:00:00Z"),
      )
    val thirdBlob =
      blobCounters(
        startBlockNumber = 21UL,
        endBlockNumber = 30UL,
        endBlockTimestamp = Instant.parse("2023-01-01T17:00:00Z"),
      )
    val fourthBlob =
      blobCounters(
        startBlockNumber = 31UL,
        endBlockNumber = 40UL,
        endBlockTimestamp = Instant.parse("2023-01-01T19:00:00Z"),
      )

    // First blob - no trigger
    calculator.newBlob(firstBlob)
    assertThat(calculator.checkAggregationTrigger(firstBlob)).isNull()

    // Second blob crosses first fork
    val firstTrigger = calculator.checkAggregationTrigger(secondBlob)
    assertThat(firstTrigger?.aggregationTriggerType).isEqualTo(AggregationTriggerType.HARD_FORK)
    assertThat(firstTrigger?.aggregation).isEqualTo(BlobsToAggregate(1UL, 10UL))

    // Reset and add more blobs
    calculator.reset()
    calculator.newBlob(secondBlob)
    calculator.newBlob(thirdBlob)

    // Fourth blob crosses second fork
    val secondTrigger = calculator.checkAggregationTrigger(fourthBlob)
    assertThat(secondTrigger?.aggregationTriggerType).isEqualTo(AggregationTriggerType.HARD_FORK)
    assertThat(secondTrigger?.aggregation).isEqualTo(BlobsToAggregate(11UL, 30UL))
  }

  @Test
  fun `should not trigger when initialized with timestamp after hard fork`() {
    val timestampAfterHardFork = Instant.parse("2023-01-01T13:00:00Z")
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(
      listOf(hardForkTimestamp),
      timestampAfterHardFork,
    )

    val firstBlob =
      blobCounters(
        startBlockNumber = 1UL,
        endBlockNumber = 10UL,
        endBlockTimestamp = Instant.parse("2023-01-01T14:00:00Z"),
      )
    val secondBlob =
      blobCounters(
        startBlockNumber = 1UL,
        endBlockNumber = 20UL,
        endBlockTimestamp = Instant.parse("2023-01-01T15:00:00Z"),
      )

    calculator.newBlob(firstBlob)
    val result = calculator.checkAggregationTrigger(secondBlob)

    assertThat(result).isNull()
  }

  @Test
  fun `should track aggregation across multiple blobs`() {
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(listOf(hardForkTimestamp), baseTimestamp)
    val firstBlob = blobCounters(startBlockNumber = 1UL, endBlockNumber = 10UL, endBlockTimestamp = beforeHardFork)
    val secondBlob = blobCounters(startBlockNumber = 1UL, endBlockNumber = 20UL, endBlockTimestamp = beforeHardFork)
    val thirdBlob = blobCounters(startBlockNumber = 21UL, endBlockNumber = 30UL, endBlockTimestamp = afterHardFork)

    calculator.newBlob(firstBlob)
    calculator.newBlob(secondBlob)
    val result = calculator.checkAggregationTrigger(thirdBlob)

    assertThat(result).isEqualTo(
      AggregationTrigger(
        aggregationTriggerType = AggregationTriggerType.HARD_FORK,
        aggregation = BlobsToAggregate(1UL, 20UL),
      ),
    )
  }

  @Test
  fun `should reset inflight aggregation but preserve timestamp tracking`() {
    val calculator = AggregationTriggerCalculatorByTimestampHardFork(listOf(hardForkTimestamp), baseTimestamp)
    val firstBlob = blobCounters(startBlockNumber = 1UL, endBlockNumber = 10UL, endBlockTimestamp = beforeHardFork)

    calculator.newBlob(firstBlob)
    calculator.reset()

    // After reset, should not have inflight aggregation
    val secondBlob = blobCounters(startBlockNumber = 1UL, endBlockNumber = 20UL, endBlockTimestamp = afterHardFork)
    val result = calculator.checkAggregationTrigger(secondBlob)
    assertThat(result).isNull()
  }
}
