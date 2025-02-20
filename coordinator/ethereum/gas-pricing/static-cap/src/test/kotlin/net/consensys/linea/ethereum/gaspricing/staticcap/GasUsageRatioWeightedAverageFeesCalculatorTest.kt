package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.Test

class GasUsageRatioWeightedAverageFeesCalculatorTest {
  private val config = GasUsageRatioWeightedAverageFeesCalculator.Config(
    baseFeeCoefficient = 0.1,
    priorityFeeCoefficient = 1.2,
    baseFeeBlobCoefficient = 0.1,
    blobSubmissionExpectedExecutionGas = 120_000,
    expectedBlobGas = 131_000
  )
  private val feesCalculator = GasUsageRatioWeightedAverageFeesCalculator(config)

  @Test
  fun calculateFees_singleBlockHistory() {
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110).map { it.toULong() },
      reward = listOf(1000).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(0.25),
      baseFeePerBlobGas = listOf(100, 110).map { it.toULong() },
      blobGasUsedRatio = listOf(0.16)
    )
    // (100*0.1 + 1000*1.2)*0.25/0.25 = 1210
    // ((100*0.1)*0.16)/0.16 * (131000.0 / 120000.0) = 10.9166666
    val executionGasPrice = (100.0 * 0.1 + 1000 * 1.2) * 0.25 / 0.25
    val blobGasPrice = (100.0 * 0.1 * 0.16 * 131_000.0) / (120_000.0 * 0.16)
    val expectedGasPrice = executionGasPrice + blobGasPrice
    val calculatedL2GasPrice = feesCalculator.calculateFees(feeHistory)
    Assertions.assertThat(expectedGasPrice.compareTo(calculatedL2GasPrice)).isZero()
  }

  @Test
  fun calculateFees_singleBlockHistory_zeroUsageRatio() {
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110).map { it.toULong() },
      reward = listOf(1000).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(0.0),
      baseFeePerBlobGas = listOf(100, 110).map { it.toULong() },
      blobGasUsedRatio = listOf(0.0)
    )
    // (100*0.1 + 1000*1.2) = 1210
    // (((100*0.1) * 0.5) / 0.5 ) * (131000.0 / 120000.0) = 10.9166666
    val executionGasPrice = 100 * 0.1 + 1000 * 1.2
    val blobGasPrice = 100 * 0.1 * 0.5 * 131000 / (0.5 * 120000)
    val expectedL2GasPrice = executionGasPrice + blobGasPrice
    val calculatedL2GasPrice = feesCalculator.calculateFees(feeHistory)
    Assertions.assertThat(calculatedL2GasPrice.compareTo(expectedL2GasPrice)).isZero()
  }

  @Test
  fun calculateFees_MultipleBlockHistory() {
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(
        0.25,
        0.5,
        0.75,
        0.9
      ),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio = listOf(
        0.16,
        0.5,
        0.66,
        0.83
      )
    )

    // Weighted Average:
    // ((100*0.1 + 1000*1.2)*0.25 + (110*0.1 + 1100*1.2)*0.50 + (120*0.1 + 1200*1.2)*0.75 + (130*0.1 + 1300*1.2)*0.9)/(0.25+0.5+0.75+0.9) = 1446.95833333
    // (((100*0.1)*0.16 + (110*0.1)*0.50 + (120*0.1)*0.66 + (130*0.1)*0.83)/(0.16+0.5+0.66+0.83)) * (131000.0 / 120000.0) = 13.10507751938
    val calculatedL2GasPrice = feesCalculator.calculateFees(feeHistory).toULong()
    Assertions.assertThat(calculatedL2GasPrice).isEqualTo(1460uL)
  }

  @Test
  fun calculateFees_equalUsageRatio_shouldEqualSimpleAverage() {
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(
        0.5,
        0.5,
        0.5,
        0.5
      ),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio = listOf(
        0.5,
        0.5,
        0.5,
        0.5
      )
    )

    // Simple Average:
    // ((100*0.1 + 1000*1.2)*0.5 + (110*0.1 + 1100*1.2)*0.5 + (120*0.1 + 1200*1.2)*0.5 + (130*0.1 + 1300*1.2)*0.5)/(0.5+0.5+0.5+0.5) = 1391.5
    // (((100*0.1)*0.5 + (110*0.1)*0.5 + (120*0.1)*0.5 + (130*0.1)*0.5)/(0.5+0.5+0.5+0.5)) * (131000.0 / 120000.0) = 12.554166666666667
    val calculatedL2GasPrice = feesCalculator.calculateFees(feeHistory).toULong()
    Assertions.assertThat(calculatedL2GasPrice).isEqualTo(1404uL)
  }

  @Test
  fun calculateFees_zeroUsage_emptyL1_blocks_should_return_SimpleAverage() {
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(
        0.0,
        0.0,
        0.0,
        0.0
      ),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio = listOf(
        0.0,
        0.0,
        0.0,
        0.0
      )
    )

    // Simple Average:
    // ((100*0.1 + 1000*1.2)*1 + (110*0.1 + 1100*1.2)*1 + (120*0.1 + 1200*1.2)*1 + (130*0.1 + 1300*1.2)*1)/4) = 1391.5
    // (((100*0.1)*0.5 + (110*0.1)*0.5 + (120*0.1)*0.5 + (130*0.1)*0.5)/(0.5+0.5+0.5+0.5)) * (131000.0 / 120000.0) = 12.554166666666667
    val calculatedL2GasPrice = feesCalculator.calculateFees(feeHistory).toULong()
    Assertions.assertThat(calculatedL2GasPrice).isEqualTo(1404uL)
  }

  @Test
  fun calculateFees_zeroUsage_emptyL1Blobs_should_return_SimpleAverage() {
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(
        0.0,
        0.0,
        0.0,
        0.0
      ),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio = listOf(
        0.0,
        0.0,
        0.0,
        0.0
      )
    )

    // Simple Average:
    // ((100*0.1 + 1000*1.2)*1 + (110*0.1 + 1100*1.2)*1 + (120*0.1 + 1200*1.2)*1 + (130*0.1 + 1300*1.2)*1)/4) = 1391.5
    // (((100*0.1)*0.5 + (110*0.1)*0.5 + (120*0.1)*0.5 + (130*0.1)*0.5)/(0.5+0.5+0.5+0.5)) * (131000.0 / 120000.0) = 12.554166666666667
    val calculatedL2GasPrice = feesCalculator.calculateFees(feeHistory).toULong()
    Assertions.assertThat(calculatedL2GasPrice).isEqualTo(1404uL)
  }

  @Test
  fun calculateFees_MultipleBlockHistory_emptyBlobLists_should_return_value() {
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(
        0.25,
        0.5,
        0.75,
        0.9
      ),
      baseFeePerBlobGas = emptyList(),
      blobGasUsedRatio = emptyList()
    )

    // Weighted Average:
    // ((100*0.1 + 1000*1.2)*0.25 + (110*0.1 + 1100*1.2)*0.50 + (120*0.1 + 1200*1.2)*0.75 + (130*0.1 + 1300*1.2)*0.9)/(0.25+0.5+0.75+0.9) = 1446.95833333
    val calculatedL2GasPrice = feesCalculator.calculateFees(feeHistory).toULong()
    Assertions.assertThat(calculatedL2GasPrice).isEqualTo(1446uL)
  }

  @Test
  fun calculateFees_partialZeroUsage_emptyL1_blocks_should_return_SimpleAverage() {
    // Very unlikely scenario in L1 network with activity.
    // Only happen for local dev or test networks, so we don't need to optimize for it.
    val feeHistory = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(
        0.75,
        0.0,
        0.75,
        0.0
      ),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio = listOf(
        0.83,
        0.0,
        0.83,
        0.0
      )
    )

    // Weighted Average:
    // ((100*0.1 + 1000*1.2)*0.75 + (110*0.1 + 1100*1.2)*0.0 + (120*0.1 + 1200*1.2)*0.75 + (130*0.1 + 1300*1.2)*0.0)/(0.75+0.0+0.75+0.0) = 1331
    // (((100*0.1)*0.83 + (110*0.1)*0.0 + (120*0.1)*0.83 + (130*0.1)*0.0)/(0.83+0.0+0.83+0.0)) * (131000.0 / 120000.0) = 12.008333333333335
    val calculatedL2GasPrice = feesCalculator.calculateFees(feeHistory).toULong()
    Assertions.assertThat(calculatedL2GasPrice).isEqualTo(1343uL)
  }
}
