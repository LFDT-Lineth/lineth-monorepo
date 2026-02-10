package net.consensys.zkevm.ethereum.coordination.conflation

import net.consensys.linea.traces.fakeTracesCountersV2
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class ConflationCalculatorByTargetBlockNumbersTest {
  private lateinit var calculator: ConflationCalculatorByTargetBlockNumbers
  private val targetBlockNumber1 = 10uL
  private val targetBlockNumber2 = 15uL

  @BeforeEach
  fun beforeEach() {
    calculator = ConflationCalculatorByTargetBlockNumbers(setOf(targetBlockNumber1, targetBlockNumber2))
  }

  @Test
  fun `checkOverflow should return overflow trigger for targetBlockNumber + 1`() {
    for (i in 1uL..targetBlockNumber1) {
      assertThat(calculator.checkOverflow(blockCounters(i))).isNull()
    }
    assertThat(calculator.checkOverflow(blockCounters(targetBlockNumber1 + 1uL)))
      .isEqualTo((ConflationCalculator.OverflowTrigger(ConflationTrigger.TARGET_BLOCK_NUMBER, false)))
    for (i in targetBlockNumber1 + 2uL..targetBlockNumber2) {
      assertThat(calculator.checkOverflow(blockCounters(i))).isNull()
    }
    assertThat(calculator.checkOverflow(blockCounters(targetBlockNumber2 + 1uL)))
      .isEqualTo((ConflationCalculator.OverflowTrigger(ConflationTrigger.TARGET_BLOCK_NUMBER, false)))
  }

  private fun blockCounters(blockNumber: ULong): BlockCounters {
    return BlockCounters(
      blockNumber = blockNumber,
      blockTimestamp = Instant.parse("2021-01-01T00:00:00.000Z"),
      tracesCounters = fakeTracesCountersV2(blockNumber.toUInt()),
      blockRLPEncoded = ByteArray(0),
    )
  }
}
