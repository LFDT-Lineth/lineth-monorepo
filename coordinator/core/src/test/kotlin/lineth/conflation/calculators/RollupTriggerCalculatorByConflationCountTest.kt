package lineth.conflation.calculators

import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.createBlock
import net.consensys.linea.traces.TracesCountersV2
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class RollupTriggerCalculatorByConflationCountTest {

  private lateinit var calculator: RollupTriggerCalculatorByConflationCount

  @BeforeEach
  fun setUp() {
    calculator = RollupTriggerCalculatorByConflationCount(maxConflations = 3)
  }

  private fun makeConflation(start: ULong, end: ULong): BlocksConflation {
    val blocks = (start..end).map { n ->
      createBlock(number = n, timestamp = Instant.fromEpochSeconds(n.toLong() * 12))
    }
    return BlocksConflation(
      blocks = blocks,
      conflationResult = ConflationCalculationResult(
        startBlockNumber = start,
        endBlockNumber = end,
        conflationTrigger = ConflationTrigger.BLOCKS_LIMIT,
        tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
      ),
    )
  }

  @Test
  fun `checkTrigger returns CONFLATION_COUNT at max conflations`() {
    val c1 = makeConflation(1UL, 2UL)
    val c2 = makeConflation(3UL, 4UL)
    val c3 = makeConflation(5UL, 6UL)

    calculator.appendConflation(c1)
    assertThat(calculator.checkTrigger(c1)).isNull()

    calculator.appendConflation(c2)
    assertThat(calculator.checkTrigger(c2)).isNull()

    calculator.appendConflation(c3)
    assertThat(calculator.checkTrigger(c3)).isEqualTo(RollupTrigger.CONFLATION_COUNT)
  }

  @Test
  fun `reset brings count back to zero so next conflation does not trigger`() {
    val c1 = makeConflation(1UL, 2UL)
    val c2 = makeConflation(3UL, 4UL)
    val c3 = makeConflation(5UL, 6UL)

    calculator.appendConflation(c1)
    calculator.appendConflation(c2)
    calculator.appendConflation(c3)
    assertThat(calculator.checkTrigger(c3)).isEqualTo(RollupTrigger.CONFLATION_COUNT)

    calculator.reset()

    val c4 = makeConflation(7UL, 8UL)
    calculator.appendConflation(c4)
    assertThat(calculator.checkTrigger(c4)).isNull()
  }
}
