package lineth.conflation.calculators

import linea.domain.BlockCounters
import linea.domain.ConflationTrigger
import net.consensys.linea.traces.TracesCountersV2
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class ConflationTriggerCalculatorByCoinbaseTest {
  private lateinit var calculator: ConflationTriggerCalculatorByCoinbase

  private val coinbaseA = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  private val coinbaseB = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

  @BeforeEach
  fun beforeEach() {
    calculator = ConflationTriggerCalculatorByCoinbase()
  }

  @Test
  fun `checkOverflow should return null when batch is empty`() {
    assertThat(calculator.checkOverflow(blockCounters(1, coinbaseA))).isNull()
  }

  @Test
  fun `checkOverflow should return null when coinbase matches the batch`() {
    calculator.appendBlock(blockCounters(1, coinbaseA))
    assertThat(calculator.checkOverflow(blockCounters(2, coinbaseA))).isNull()
  }

  @Test
  fun `checkOverflow should trigger when coinbase changes`() {
    calculator.appendBlock(blockCounters(1, coinbaseA))
    assertThat(calculator.checkOverflow(blockCounters(2, coinbaseB)))
      .isEqualTo(ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.COINBASE_CHANGE, false))
  }

  @Test
  fun `appendBlock should throw when coinbase differs from batch coinbase`() {
    calculator.appendBlock(blockCounters(1, coinbaseA))
    assertThatThrownBy { calculator.appendBlock(blockCounters(2, coinbaseB)) }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("coinbase")
  }

  @Test
  fun `reset should clear batch coinbase so next block starts a fresh batch`() {
    calculator.appendBlock(blockCounters(1, coinbaseA))
    calculator.reset()
    assertThat(calculator.checkOverflow(blockCounters(2, coinbaseB))).isNull()
    calculator.appendBlock(blockCounters(2, coinbaseB))
    assertThat(calculator.checkOverflow(blockCounters(3, coinbaseA)))
      .isEqualTo(ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.COINBASE_CHANGE, false))
  }

  @Test
  fun `should accumulate multiple blocks with the same coinbase without triggering`() {
    repeat(10) { i ->
      assertThat(calculator.checkOverflow(blockCounters(i + 1, coinbaseA))).isNull()
      calculator.appendBlock(blockCounters(i + 1, coinbaseA))
    }
    assertThat(calculator.checkOverflow(blockCounters(11, coinbaseB)))
      .isEqualTo(ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.COINBASE_CHANGE, false))
  }

  private fun blockCounters(blockNumber: Int, coinbase: String): BlockCounters =
    BlockCounters(
      blockNumber = blockNumber.toULong(),
      blockTimestamp = Instant.parse("2021-01-01T00:00:00.000Z"),
      tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
      blockRLPEncoded = ByteArray(0),
      coinbase = coinbase,
    )
}
