/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import java.math.BigInteger

class ConstantDifficultyTerminalBlockCalculatorTest {
  private fun calculator(
    chain: FakeElChain,
    terminalTotalDifficulty: Long,
  ) = ConstantDifficultyTerminalBlockCalculator(chain, BigInteger.valueOf(terminalTotalDifficulty))

  /** Brute force: first block whose summed difficulty from genesis reaches TTD. */
  private fun summedTerminalBlock(
    difficulties: List<BigInteger>,
    terminalTotalDifficulty: Long,
  ): ULong {
    var totalDifficulty = BigInteger.ZERO
    difficulties.forEachIndexed { number, difficulty ->
      totalDifficulty += difficulty
      if (totalDifficulty >= BigInteger.valueOf(terminalTotalDifficulty)) return number.toULong()
    }
    error("TTD not reached")
  }

  @Test
  fun `QBFT chain reaches TTD at block TTD minus genesis difficulty`() {
    // genesis 0x1, every block difficulty 1 => TD(n) = 1 + n
    val chain = FakeElChain().extend(1).extendRepeated(count = 2, difficulty = 1)

    val terminalBlock = calculator(chain, terminalTotalDifficulty = 6).terminalBlock()!!

    assertThat(terminalBlock.number).isEqualTo(5UL)
    assertThat(terminalBlock.genesisDifficulty).isEqualTo(BigInteger.ONE)
    assertThat(terminalBlock.blockDifficulty).isEqualTo(BigInteger.ONE)
  }

  @Test
  fun `single-signer Clique chain rounds up to the first block reaching TTD`() {
    // genesis 0x1, every block in-turn with difficulty 2 => TD(n) = 1 + 2n
    val chain = FakeElChain().extend(1, 2)

    // TD(4) = 9 < 10, TD(5) = 11 >= 10
    assertThat(calculator(chain, terminalTotalDifficulty = 10).terminalBlock()!!.number).isEqualTo(5UL)
    // TD(5) = 11 exactly
    assertThat(calculator(chain, terminalTotalDifficulty = 11).terminalBlock()!!.number).isEqualTo(5UL)
    // TD(5) = 11 < 12, TD(6) = 13
    assertThat(calculator(chain, terminalTotalDifficulty = 12).terminalBlock()!!.number).isEqualTo(6UL)
  }

  @Test
  fun `matches summing difficulties block by block for constant difficulty chains`() {
    for (genesisDifficulty in listOf(0L, 1L, 7L, 0x400000000L)) {
      for (blockDifficulty in listOf(1L, 2L, 3L)) {
        for (ttdOffset in 1L..20L) {
          val terminalTotalDifficulty = genesisDifficulty + ttdOffset
          val chain = FakeElChain().extend(genesisDifficulty).extendRepeated(count = 30, difficulty = blockDifficulty)

          assertThat(calculator(chain, terminalTotalDifficulty).terminalBlock()!!.number)
            .describedAs("g=$genesisDifficulty d=$blockDifficulty ttd=$terminalTotalDifficulty")
            .isEqualTo(summedTerminalBlock(chain.difficulties, terminalTotalDifficulty))
        }
      }
    }
  }

  @Test
  fun `terminal block is genesis when genesis difficulty already reaches TTD`() {
    val chain = FakeElChain().extend(10)

    val terminalBlock = calculator(chain, terminalTotalDifficulty = 10).terminalBlock()!!

    assertThat(terminalBlock.number).isEqualTo(0UL)
    assertThat(terminalBlock.blockDifficulty).isNull()
    // block 1 is not needed
    assertThat(chain.requestedBlockNumbers).containsExactly(0UL)
  }

  @Test
  fun `returns null until block 1 exists, then computes the terminal block`() {
    val chain = FakeElChain().extend(1)
    val calculator = calculator(chain, terminalTotalDifficulty = 6)

    assertThat(calculator.terminalBlock()).isNull()

    chain.extend(1)
    assertThat(calculator.terminalBlock()!!.number).isEqualTo(5UL)
  }

  @Test
  fun `returns null when the EL has no blocks`() {
    assertThat(calculator(FakeElChain(), terminalTotalDifficulty = 6).terminalBlock()).isNull()
  }

  @Test
  fun `computes the terminal block only once`() {
    val chain = FakeElChain().extend(1, 1)
    val calculator = calculator(chain, terminalTotalDifficulty = 6)

    repeat(3) { calculator.terminalBlock() }

    assertThat(chain.requestedBlockNumbers).containsExactly(0UL, 1UL)
  }

  @Test
  fun `fails when block 1 is post-merge while genesis is below TTD`() {
    val chain = FakeElChain().extend(1, 0)

    assertThatThrownBy { calculator(chain, terminalTotalDifficulty = 6).terminalBlock() }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("Maru TTD does not match the EL TTD")
  }
}
