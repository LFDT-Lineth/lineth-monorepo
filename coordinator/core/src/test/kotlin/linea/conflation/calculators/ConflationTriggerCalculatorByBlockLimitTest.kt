package linea.conflation.calculators

import linea.domain.BlockCounters
import linea.domain.ConflationTrigger
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.fakeTracesCountersV2
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class ConflationTriggerCalculatorByBlockLimitTest {
  private lateinit var calculator: ConflationTriggerCalculatorByBlockLimit

  @BeforeEach
  fun beforeEach() {
    calculator = ConflationTriggerCalculatorByBlockLimit(3u)
  }

  @Test
  fun `should accumulate blockCounting`() {
    val counters = ConflationCounters.empty(TracesCountersV2.EMPTY_TRACES_COUNT)
    calculator.copyCountersTo(counters)
    assertThat(counters.blockCount).isEqualTo(0u)
    calculator.appendBlock(blockCounters(1))
    calculator.appendBlock(blockCounters(2))
    calculator.appendBlock(blockCounters(3))
    calculator.copyCountersTo(counters)
    assertThat(counters.blockCount).isEqualTo(3u)

    calculator.reset()
    calculator.copyCountersTo(counters)
    assertThat(counters.blockCount).isEqualTo(0u)
  }

  @Test
  fun `checkOverflow should return overflow trigger`() {
    calculator.appendBlock(blockCounters(10))
    calculator.appendBlock(blockCounters(20))
    assertThat(calculator.checkOverflow(blockCounters(30))).isNull()
    calculator.appendBlock(blockCounters(30))

    assertThat(calculator.checkOverflow(blockCounters(40)))
      .isEqualTo((ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.BLOCKS_LIMIT, false)))
  }

  private fun blockCounters(blockNumber: Int): BlockCounters {
    return BlockCounters(
      blockNumber = blockNumber.toULong(),
      blockTimestamp = Instant.parse("2021-01-01T00:00:00.000Z"),
      tracesCounters = fakeTracesCountersV2(blockNumber.toUInt()),
      blockRLPEncoded = ByteArray(0),
    )
  }
}
