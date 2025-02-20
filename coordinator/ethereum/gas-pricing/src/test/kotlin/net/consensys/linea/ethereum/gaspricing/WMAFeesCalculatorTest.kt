package net.consensys.linea.ethereum.gaspricing

import linea.domain.FeeHistory
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class WMAFeesCalculatorTest {
  @Test
  fun `calculateFees weighted moving average of priority fee`() {
    val wmaFeesCalculator =
      WMAFeesCalculator(
        WMAFeesCalculator.Config(
          baseFeeCoefficient = 0.1,
          priorityFeeWmaCoefficient = 1.2
        )
      )

    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(0.25, 0.5, 0.75, 0.9),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio = listOf(0.25, 0.5, 0.75, 0.9)
    )

    // WMA = (140*0.1) + ((1000*0.25*1 + 1100*0.5*2 + 1200*0.75*3 + 1300*0.9*4) / (0.25*1 + 0.5*2 + 0.75*3 + 0.9*4)) * 1.2 = 1489.49295
    val expectedBaseFee = 140 * 0.1
    val expectedPriorityFee = (
      (1000 * 0.25 * 1 + 1100 * 0.5 * 2 + 1200 * 0.75 * 3 + 1300 * 0.9 * 4) /
        (0.25 * 1 + 0.5 * 2 + 0.75 * 3 + 0.9 * 4)
      ) * 1.2

    val expectedL2GasPrice = expectedBaseFee + expectedPriorityFee
    val calculatedL2GasPrice = wmaFeesCalculator.calculateFees(feeHistory)
    assertThat(calculatedL2GasPrice).isEqualTo(expectedL2GasPrice)
  }

  @Test
  fun `calculateFees supports empty usage ratio`() {
    val wmaFeesCalculator =
      WMAFeesCalculator(
        WMAFeesCalculator.Config(
          baseFeeCoefficient = 0.1,
          priorityFeeWmaCoefficient = 1.2
        )
      )

    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(0.0, 0.0, 0.0, 0.0),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio = listOf(0.0, 0.0, 0.0, 0.0)
    )

    // WMA = (140*0.1) = 14.0
    val calculatedL2GasPrice = wmaFeesCalculator.calculateFees(feeHistory)
    val expectedL2GasPrice = 140 * 0.1
    assertThat(calculatedL2GasPrice).isEqualTo(expectedL2GasPrice)
  }
}
