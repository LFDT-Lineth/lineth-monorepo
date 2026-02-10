package net.consensys.zkevm.ethereum.coordination.conflation

import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.fakeTracesCountersV2
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class TimestampHardForkConflationCalculatorTest {
  private val baseTimestamp = Instant.parse("2023-01-01T10:00:00Z")
  private val hardForkTimestamp = Instant.parse("2023-01-01T12:00:00Z")
  private val beforeHardFork = Instant.parse("2023-01-01T11:59:59Z")
  private val afterHardFork = Instant.parse("2023-01-01T12:00:01Z")

  @Test
  fun `should have correct id`() {
    val calculator = TimestampHardForkConflationCalculator(listOf(hardForkTimestamp), baseTimestamp)
    assertThat(calculator.id).isEqualTo("TIMESTAMP_HARD_FORK")
  }

  @Test
  fun `should require non-empty timestamps in constructor`() {
    assertThatThrownBy {
      TimestampHardForkConflationCalculator(emptyList(), baseTimestamp)
    }.isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `should require unique timestamps in constructor`() {
    val duplicatedTimestamps = listOf(
      Instant.parse("2023-01-01T12:00:00Z"),
      Instant.parse("2023-01-01T12:00:00Z"), // Duplicate
      Instant.parse("2023-01-01T14:00:00Z"),
    )
    assertThatThrownBy {
      TimestampHardForkConflationCalculator(duplicatedTimestamps, baseTimestamp)
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
      TimestampHardForkConflationCalculator(unsortedTimestamps, baseTimestamp)
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
    val calculator = TimestampHardForkConflationCalculator(sortedTimestamps, baseTimestamp)
    assertThat(calculator.id).isEqualTo("TIMESTAMP_HARD_FORK")
  }

  @Test
  fun `should not trigger overflow when block timestamp is before hard fork`() {
    val calculator = TimestampHardForkConflationCalculator(listOf(hardForkTimestamp), baseTimestamp)
    val blockCounters = blockCounters(blockNumber = 1, timestamp = beforeHardFork)

    val result = calculator.checkOverflow(blockCounters)

    assertThat(result).isNull()
  }

  @Test
  fun `should trigger overflow when block timestamp equals hard fork timestamp`() {
    val calculator = TimestampHardForkConflationCalculator(listOf(hardForkTimestamp), baseTimestamp)
    val blockCounters = blockCounters(blockNumber = 100, timestamp = hardForkTimestamp)

    val result = calculator.checkOverflow(blockCounters)

    assertThat(result).isEqualTo(
      ConflationCalculator.OverflowTrigger(
        trigger = ConflationTrigger.HARD_FORK,
        singleBlockOverSized = false,
      ),
    )
  }

  @Test
  fun `should trigger overflow when block timestamp is after hard fork timestamp`() {
    val calculator = TimestampHardForkConflationCalculator(listOf(hardForkTimestamp), baseTimestamp)
    val blockCounters = blockCounters(blockNumber = 101, timestamp = afterHardFork)

    val result = calculator.checkOverflow(blockCounters)

    assertThat(result).isEqualTo(
      ConflationCalculator.OverflowTrigger(
        trigger = ConflationTrigger.HARD_FORK,
        singleBlockOverSized = false,
      ),
    )
  }

  @Test
  fun `should not trigger when initialized with timestamp after hard fork`() {
    val timestampAfterHardFork = Instant.parse("2023-01-01T13:00:00Z")
    val calculator = TimestampHardForkConflationCalculator(
      listOf(hardForkTimestamp),
      timestampAfterHardFork,
    )

    // Block after hard fork should not trigger since we already passed it
    val blockCounters = blockCounters(blockNumber = 1, timestamp = Instant.parse("2023-01-01T14:00:00Z"))
    val result = calculator.checkOverflow(blockCounters)

    assertThat(result).isNull()
  }

  @Test
  fun `should trigger when initialized with timestamp before hard fork`() {
    val timestampBeforeHardFork = Instant.parse("2023-01-01T11:00:00Z")
    val calculator = TimestampHardForkConflationCalculator(
      listOf(hardForkTimestamp),
      timestampBeforeHardFork,
    )

    // Block at hard fork should trigger since we haven't passed it yet
    val blockCounters = blockCounters(blockNumber = 1, timestamp = hardForkTimestamp)
    val result = calculator.checkOverflow(blockCounters)

    assertThat(result).isEqualTo(
      ConflationCalculator.OverflowTrigger(
        trigger = ConflationTrigger.HARD_FORK,
        singleBlockOverSized = false,
      ),
    )
  }

  @Test
  fun `should prevent duplicate triggers on restart scenario`() {
    // Simulate coordinator restart after hard fork has already passed
    val currentTimestamp = Instant.parse("2023-01-01T15:00:00Z") // Well after hard fork
    val calculator = TimestampHardForkConflationCalculator(
      listOf(hardForkTimestamp),
      currentTimestamp,
    )

    // Process a block after the hard fork - should not trigger
    val blockCounters = blockCounters(blockNumber = 1000, timestamp = Instant.parse("2023-01-01T16:00:00Z"))
    val result = calculator.checkOverflow(blockCounters)

    assertThat(result).isNull()
  }

  @Test
  fun `should handle multiple hard fork timestamps`() {
    val firstFork = Instant.parse("2023-01-01T12:00:00Z")
    val secondFork = Instant.parse("2023-01-01T18:00:00Z")
    val thirdFork = Instant.parse("2023-01-02T00:00:00Z")
    val calculator = TimestampHardForkConflationCalculator(
      listOf(firstFork, secondFork, thirdFork),
      baseTimestamp,
    )

    // Block before first fork - should not trigger
    val beforeFirstFork = blockCounters(blockNumber = 1, timestamp = Instant.parse("2023-01-01T11:59:59Z"))
    assertThat(calculator.checkOverflow(beforeFirstFork)).isNull()
    calculator.appendBlock(beforeFirstFork)

    // Block at first fork - should trigger
    val atFirstFork = blockCounters(blockNumber = 2, timestamp = firstFork)
    val firstResult = calculator.checkOverflow(atFirstFork)
    assertThat(firstResult?.trigger).isEqualTo(ConflationTrigger.HARD_FORK)
    calculator.appendBlock(atFirstFork)

    // Block between first and second fork - should not trigger
    val betweenForks = blockCounters(blockNumber = 3, timestamp = Instant.parse("2023-01-01T15:00:00Z"))
    assertThat(calculator.checkOverflow(betweenForks)).isNull()
    calculator.appendBlock(betweenForks)

    // Block at second fork - should trigger
    val atSecondFork = blockCounters(blockNumber = 4, timestamp = secondFork)
    val secondResult = calculator.checkOverflow(atSecondFork)
    assertThat(secondResult?.trigger).isEqualTo(ConflationTrigger.HARD_FORK)
    calculator.appendBlock(atSecondFork)
  }

  @Test
  fun `should only trigger on the first block that crosses a hard fork boundary`() {
    val calculator = TimestampHardForkConflationCalculator(listOf(hardForkTimestamp), baseTimestamp)

    // Process blocks before the hard fork
    val beforeFork = blockCounters(blockNumber = 1, timestamp = Instant.parse("2023-01-01T11:59:59Z"))
    calculator.appendBlock(beforeFork)
    assertThat(calculator.checkOverflow(beforeFork)).isNull()

    // First block at/after hard fork should trigger
    val atFork = blockCounters(blockNumber = 2, timestamp = hardForkTimestamp)
    val firstResult = calculator.checkOverflow(atFork)
    assertThat(firstResult?.trigger).isEqualTo(ConflationTrigger.HARD_FORK)
    calculator.appendBlock(atFork)

    // Subsequent blocks after the same hard fork should not trigger
    val afterFork = blockCounters(blockNumber = 3, timestamp = Instant.parse("2023-01-01T12:00:01Z"))
    val secondResult = calculator.checkOverflow(afterFork)
    assertThat(secondResult).isNull()
  }

  @Test
  fun `reset should not reset last processed timestamp`() {
    val calculator = TimestampHardForkConflationCalculator(listOf(hardForkTimestamp), baseTimestamp)

    // Process a block at hard fork
    val atFork = blockCounters(blockNumber = 1, timestamp = hardForkTimestamp)
    calculator.appendBlock(atFork)
    calculator.checkOverflow(atFork)

    // Reset
    calculator.reset()

    // Should not trigger again for same hard fork
    val afterReset = blockCounters(blockNumber = 2, timestamp = Instant.parse("2023-01-01T12:00:01Z"))
    val result = calculator.checkOverflow(afterReset)
    assertThat(result).isNull()
  }

  @Test
  fun `copyCountersTo should not modify counters`() {
    val calculator = TimestampHardForkConflationCalculator(listOf(hardForkTimestamp), baseTimestamp)
    val counters = ConflationCounters.empty(TracesCountersV2.EMPTY_TRACES_COUNT)
    val originalBlockCount = counters.blockCount

    calculator.copyCountersTo(counters)

    assertThat(counters.blockCount).isEqualTo(originalBlockCount)
  }

  private fun blockCounters(blockNumber: Int, timestamp: Instant): BlockCounters {
    return BlockCounters(
      blockNumber = blockNumber.toULong(),
      blockTimestamp = timestamp,
      tracesCounters = fakeTracesCountersV2(blockNumber.toUInt()),
      blockRLPEncoded = ByteArray(0),
    )
  }
}
