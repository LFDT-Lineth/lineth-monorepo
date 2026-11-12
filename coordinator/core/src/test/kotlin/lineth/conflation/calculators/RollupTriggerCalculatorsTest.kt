package lineth.conflation.calculators

import linea.da.DaStreamPosition
import lineth.conflation.calculators.DaTestConflations.conflation
import lineth.conflation.calculators.DaTestConflations.encoder
import lineth.conflation.calculators.DaTestConflations.genesisDataRollingHash
import lineth.coordination.riscv.da.FakeDaStream
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class RollupTriggerCalculatorsTest {
  private val stream = FakeDaStream(DaStreamPosition(genesisDataRollingHash))

  private fun compressed(
    startNumber: ULong,
    endNumber: ULong,
    frameSize: Int = 20_000,
    gasUsed: ULong = 1_000UL,
    txsPerBlock: Int = 0,
  ): CompressedBlocksConflation {
    val conflation = conflation(startNumber, endNumber, frameSize, gasUsed = gasUsed, txsPerBlock = txsPerBlock)
    return CompressedBlocksConflation(conflation, stream.compress(conflation.blocks.map(encoder::encode)))
  }

  @Test
  fun `data limit sums frames across conflations, allows reaching the limit, and resets`() {
    val calculator = RollupTriggerCalculatorByDataLimit(60_000UL)
    calculator.appendConflation(compressed(1UL, 3UL, frameSize = 20_000))
    calculator.appendConflation(compressed(4UL, 5UL, frameSize = 25_000))

    // 45k so far: 15k reaches the limit exactly, 15k + 1 byte exceeds it.
    assertThat(calculator.checkOverflow(compressed(6UL, 8UL, frameSize = 15_000))).isNull()
    assertThat(calculator.checkOverflow(compressed(6UL, 8UL, frameSize = 15_001))).isEqualTo(RollupTrigger.DATA_LIMIT)
    assertThat(calculator.checkTrigger(compressed(6UL, 8UL).conflation)).isNull()
    calculator.reset()
    assertThat(calculator.checkOverflow(compressed(6UL, 8UL, frameSize = 60_000))).isNull()
  }

  @Test
  fun `gas limit sums the gas used of every block in every conflation`() {
    val calculator = RollupTriggerCalculatorByGas(10_000UL)
    calculator.appendConflation(compressed(1UL, 3UL, gasUsed = 1_000UL)) // 3 blocks: 3_000
    calculator.appendConflation(compressed(4UL, 7UL, gasUsed = 1_000UL)) // 4 blocks: 4_000

    // 7_000 so far: 3 blocks of 1_000 reach the limit exactly, a 4th block exceeds it although each block is small.
    assertThat(calculator.checkOverflow(compressed(8UL, 10UL, gasUsed = 1_000UL))).isNull()
    assertThat(calculator.checkOverflow(compressed(8UL, 11UL, gasUsed = 1_000UL))).isEqualTo(RollupTrigger.GAS)
    calculator.reset()
    assertThat(calculator.checkOverflow(compressed(8UL, 17UL, gasUsed = 1_000UL))).isNull()
  }

  @Test
  fun `tx count limit sums the transactions of every block in every conflation`() {
    val calculator = RollupTriggerCalculatorByTxCount(10UL)
    calculator.appendConflation(compressed(1UL, 2UL, txsPerBlock = 2)) // 2 blocks: 4 txs
    calculator.appendConflation(compressed(3UL, 3UL)) // no txs

    // 4 so far: 3 blocks of 2 reach the limit exactly, 3 blocks of 2 plus 1 more block exceed it.
    assertThat(calculator.checkOverflow(compressed(4UL, 6UL, txsPerBlock = 2))).isNull()
    assertThat(calculator.checkOverflow(compressed(4UL, 7UL, txsPerBlock = 2))).isEqualTo(RollupTrigger.TX_COUNT)
    calculator.reset()
    assertThat(calculator.checkOverflow(compressed(4UL, 8UL, txsPerBlock = 2))).isNull()
  }

  @Test
  fun `sum triggers need the compressed conflation`() {
    assertThatThrownBy { RollupTriggerCalculatorByDataLimit(1UL).appendConflation(compressed(1UL, 2UL).conflation) }
      .isInstanceOf(UnsupportedOperationException::class.java)
  }

  @Test
  fun `target block numbers end the rollup after a conflation ending at a target`() {
    val calculator = RollupTriggerCalculatorByTargetBlockNumbers(setOf(5UL))

    // 5 inside 4..6 does not end the rollup; only a conflation ending at 5 does.
    assertThat(calculator.checkTrigger(compressed(1UL, 3UL).conflation)).isNull()
    assertThat(calculator.checkTrigger(compressed(4UL, 6UL).conflation)).isNull()
    assertThat(calculator.checkTrigger(compressed(4UL, 5UL).conflation)).isEqualTo(RollupTrigger.TARGET_BLOCK_NUMBER)
  }
}
