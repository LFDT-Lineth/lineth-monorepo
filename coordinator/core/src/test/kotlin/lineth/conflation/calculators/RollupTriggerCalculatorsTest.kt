package lineth.conflation.calculators

import linea.da.DaStreamPosition
import lineth.conflation.calculators.DaPipelineTestSupport.conflation
import lineth.conflation.calculators.DaPipelineTestSupport.encoder
import lineth.conflation.calculators.DaPipelineTestSupport.genesisDataRollingHash
import lineth.coordination.riscv.da.FakeDaStream
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class RollupTriggerCalculatorsTest {
  private val stream = FakeDaStream(DaStreamPosition(genesisDataRollingHash))

  private fun compressed(number: ULong, frameSize: Int, gasUsed: ULong = 1_000UL): CompressedBlocksConflation {
    val conflation = conflation(number, frameSize, gasUsed = gasUsed)
    return CompressedBlocksConflation(conflation, stream.compress(conflation.blocks.map(encoder::encode)))
  }

  @Test
  fun `data limit ends the rollup before a conflation that would exceed it, and resets`() {
    val calculator = RollupTriggerCalculatorByDataLimit(25_000UL)
    calculator.appendConflation(compressed(1UL, 10_000))

    assertThat(calculator.checkOverflow(compressed(2UL, 15_000))).isNull()
    assertThat(calculator.checkOverflow(compressed(2UL, 15_001))).isEqualTo(RollupTrigger.DATA_LIMIT)
    calculator.reset()
    assertThat(calculator.checkOverflow(compressed(2UL, 20_000))).isNull()
    assertThat(calculator.checkTrigger(compressed(2UL, 20_000).conflation)).isNull()
  }

  @Test
  fun `gas limit sums the blocks' gas used`() {
    val calculator = RollupTriggerCalculatorByGas(3_000UL)
    calculator.appendConflation(compressed(1UL, 1_000, gasUsed = 2_000UL))

    assertThat(calculator.checkOverflow(compressed(2UL, 1_000, gasUsed = 1_000UL))).isNull()
    assertThat(calculator.checkOverflow(compressed(2UL, 1_000, gasUsed = 1_001UL))).isEqualTo(RollupTrigger.GAS)
  }

  @Test
  fun `tx count limit sums the blocks' transactions`() {
    val calculator = RollupTriggerCalculatorByTxCount(0UL)
    calculator.appendConflation(compressed(1UL, 1_000))

    // Fixture blocks have no transactions, so the count never exceeds 0.
    assertThat(calculator.checkOverflow(compressed(2UL, 1_000))).isNull()
  }

  @Test
  fun `sum triggers need the compressed conflation`() {
    assertThatThrownBy { RollupTriggerCalculatorByDataLimit(1UL).appendConflation(compressed(1UL, 1_000).conflation) }
      .isInstanceOf(UnsupportedOperationException::class.java)
  }

  @Test
  fun `target block numbers end the rollup after a conflation ending at a target`() {
    val calculator = RollupTriggerCalculatorByTargetBlockNumbers(setOf(2UL))

    assertThat(calculator.checkTrigger(compressed(1UL, 1_000).conflation)).isNull()
    assertThat(calculator.checkTrigger(compressed(2UL, 1_000).conflation)).isEqualTo(RollupTrigger.TARGET_BLOCK_NUMBER)
  }
}
