package net.consensys.linea

import linea.domain.FeeHistory
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class FeeHistoryTest {

  @Test
  fun blocksRange() {
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(1uL, 2uL, 3uL, 4uL),
      reward = listOf(listOf(1uL), listOf(2uL), listOf(3uL)),
      gasUsedRatio = listOf(0.1, 0.2, 0.3),
      baseFeePerBlobGas = listOf(1uL, 2uL, 3uL, 4uL),
      blobGasUsedRatio = listOf(0.1, 0.2, 0.3)
    )
    assertThat(feeHistory.blocksRange()).isEqualTo(100uL..102uL)
  }
}
