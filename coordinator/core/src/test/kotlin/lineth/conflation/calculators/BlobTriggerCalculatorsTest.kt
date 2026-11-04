package lineth.conflation.calculators

import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.createBlock
import net.consensys.linea.traces.TracesCountersV5
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class BlobTriggerCalculatorsTest {
  private val baseTime = Instant.parse("2026-10-01T00:00:00Z")

  private fun conflation(start: ULong, end: ULong, firstTimestamp: Instant = baseTime): BlocksConflation =
    BlocksConflation(
      (start..end).mapIndexed { i, n -> createBlock(number = n, timestamp = firstTimestamp + (i * 2).seconds) },
      ConflationCalculationResult(start, end, ConflationTrigger.BLOCKS_LIMIT, TracesCountersV5.EMPTY_TRACES_COUNT),
    )

  @Test
  fun `target block numbers trigger after a conflation ending at a target`() {
    val calculator = BlobTriggerCalculatorByTargetBlockNumbers(setOf(10UL, 20UL))

    assertThat(calculator.checkTrigger(conflation(5UL, 10UL))).isEqualTo(BlobTrigger.TARGET_BLOCK_NUMBER)
    assertThat(calculator.checkTrigger(conflation(11UL, 15UL))).isNull()
    // A conflation overlapping a target (not ending at it) is a misconfiguration: logged, no trigger.
    assertThat(calculator.checkTrigger(conflation(18UL, 25UL))).isNull()
    assertThat(calculator.checkOverflow(conflation(20UL, 20UL))).isNull()
  }

  @Test
  fun `a hard fork triggers before the first conflation at or after the fork`() {
    val fork = baseTime + 100.seconds
    val calculator = BlobTriggerCalculatorByHardForkTimestamp(listOf(fork), initialTimestamp = baseTime)
    val beforeFork = conflation(1UL, 3UL, firstTimestamp = baseTime + 10.seconds)
    val atFork = conflation(4UL, 5UL, firstTimestamp = fork)
    val afterFork = conflation(6UL, 7UL, firstTimestamp = fork + 20.seconds)

    assertThat(calculator.checkOverflow(beforeFork)).isNull()
    calculator.appendConflation(beforeFork)
    assertThat(calculator.checkOverflow(atFork)).isEqualTo(BlobTrigger.HARD_FORK)
    calculator.appendConflation(atFork)
    calculator.reset()
    assertThat(calculator.checkOverflow(afterFork)).isNull()
    assertThat(calculator.checkTrigger(afterFork)).isNull()
  }

  @Test
  fun `hard fork timestamps must be non-empty and sorted`() {
    assertThatThrownBy { BlobTriggerCalculatorByHardForkTimestamp(emptyList(), baseTime) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { BlobTriggerCalculatorByHardForkTimestamp(listOf(baseTime + 2.seconds, baseTime), baseTime) }
      .isInstanceOf(IllegalArgumentException::class.java)
  }
}
