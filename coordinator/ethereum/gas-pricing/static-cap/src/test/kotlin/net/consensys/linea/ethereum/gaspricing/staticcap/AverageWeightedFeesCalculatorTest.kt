package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import org.mockito.kotlin.mock
import java.util.stream.Stream

class AverageWeightedFeesCalculatorTest {

  @ParameterizedTest(name = "{0}")
  @MethodSource("averageWeightedFeesCalculatorTestCases")
  fun `test AverageWeightedFeesCalculator`(
    @Suppress("UNUSED_PARAMETER") name: String,
    expectedWeightedAverage: Double,
    feeHistory: FeeHistory
  ) {
    val mockLog = mock<Logger>()
    val feeList = { fh: FeeHistory -> fh.baseFeePerGas }
    val ratioList = { fh: FeeHistory -> fh.gasUsedRatio }
    val actualWeightedAverage = AverageWeightedFeesCalculator(feeList, ratioList, mockLog)
      .calculateFees(feeHistory)
    assertThat(actualWeightedAverage).isEqualTo(expectedWeightedAverage)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("averageWeightedBaseFeesCalculatorTestCases")
  fun `test AverageWeightedBaseFeesCalculator`(
    @Suppress("UNUSED_PARAMETER") name: String,
    expectedWeightedAverage: Double,
    feeHistory: FeeHistory
  ) {
    val actualWeightedAverage = AverageWeightedBaseFeesCalculator.calculateFees(feeHistory)
    assertThat(actualWeightedAverage).isEqualTo(expectedWeightedAverage)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("averageWeightedPriorityFeesCalculatorTestCases")
  fun `test AverageWeightedPriorityFeesCalculator`(
    @Suppress("UNUSED_PARAMETER") name: String,
    expectedWeightedAverage: Double,
    feeHistory: FeeHistory
  ) {
    val actualWeightedAverage = AverageWeightedPriorityFeesCalculator.calculateFees(feeHistory)
    assertThat(actualWeightedAverage).isEqualTo(expectedWeightedAverage)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("averageWeightedBlobBaseFeesCalculatorTestCases")
  fun `test AverageWeightedBlobBaseFeesCalculator`(
    @Suppress("UNUSED_PARAMETER") name: String,
    expectedWeightedAverage: Double,
    feeHistory: FeeHistory
  ) {
    val actualWeightedAverage = AverageWeightedBlobBaseFeesCalculator.calculateFees(feeHistory)
    assertThat(actualWeightedAverage).isEqualTo(expectedWeightedAverage)
  }

  companion object {

    data class FeeHistoryTestCase(
      val name: String,
      val feeHistory: FeeHistory,
      val weightedAverageBaseFee: Double,
      val weightedAveragePriorityFee: Double,
      val weightedAverageBlobBaseFee: Double
    )

    private val feeHistoryTestCases = listOf(
      FeeHistoryTestCase(
        name = "Single block history",
        feeHistory = FeeHistory(
          oldestBlock = 100uL,
          baseFeePerGas = listOf(100, 110).map { it.toULong() },
          reward = listOf(1000).map { listOf(it.toULong()) },
          gasUsedRatio = listOf(0.25).map { it },
          baseFeePerBlobGas = listOf(200, 210).map { it.toULong() },
          blobGasUsedRatio = listOf(0.16).map { it }
        ),
        weightedAverageBaseFee = 100.0, // 100*0.25/0.25
        weightedAveragePriorityFee = 1000.0, // 1000*0.25/0.25
        weightedAverageBlobBaseFee = 200.0 // 200*0.16/0.16
      ),
      FeeHistoryTestCase(
        name = "Single block history with zero ratio sum -> Simple average",
        feeHistory = FeeHistory(
          oldestBlock = 100uL,
          baseFeePerGas = listOf(100, 110).map { it.toULong() },
          reward = listOf(1000).map { listOf(it.toULong()) },
          gasUsedRatio = listOf(0.0).map { it },
          baseFeePerBlobGas = listOf(200, 210).map { it.toULong() },
          blobGasUsedRatio = listOf(0.0).map { it }
        ),
        weightedAverageBaseFee = 100.0,
        weightedAveragePriorityFee = 1000.0,
        weightedAverageBlobBaseFee = 200.0
      ),
      FeeHistoryTestCase(
        name = "Multiple block history",
        feeHistory = FeeHistory(
          oldestBlock = 100uL,
          baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
          reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
          gasUsedRatio = listOf(0.25, 0.5, 0.75, 0.9).map { it },
          baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
          blobGasUsedRatio = listOf(0.16, 0.5, 0.66, 0.83).map { it }
        ),
        weightedAverageBaseFee = listOf(100, 110, 120, 130, 140).map { it }
          .zip(listOf(0.25, 0.5, 0.75, 0.9).map { it })
          .sumOf { it.first * it.second } /
          (
            listOf(0.25, 0.5, 0.75, 0.9).map { it }.sumOf { it }
            ),
        weightedAveragePriorityFee = listOf(1000, 1100, 1200, 1300, 1400).map { it }
          .zip(listOf(0.25, 0.5, 0.75, 0.9).map { it })
          .sumOf { it.first * it.second } /
          (
            listOf(0.25, 0.5, 0.75, 0.9).map { it }.sumOf { it }
            ),
        weightedAverageBlobBaseFee = listOf(100, 110, 120, 130, 140).map { it }
          .zip(listOf(0.16, 0.5, 0.66, 0.83).map { it })
          .sumOf { it.first * it.second } /
          (
            listOf(0.16, 0.5, 0.66, 0.83).map { it }.sumOf { it }
            )
      ),
      FeeHistoryTestCase(
        name = "Multiple block history equal ratio -> Simple Average",
        feeHistory = FeeHistory(
          oldestBlock = 100uL,
          baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
          reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
          gasUsedRatio = listOf(0.25, 0.25, 0.25, 0.25).map { it },
          baseFeePerBlobGas = listOf(200, 210, 220, 230, 240).map { it.toULong() },
          blobGasUsedRatio = listOf(0.16, 0.16, 0.16, 0.16).map { it }
        ),
        weightedAverageBaseFee = 115.0,
        weightedAveragePriorityFee = 1150.0,
        weightedAverageBlobBaseFee = 215.0
      ),
      FeeHistoryTestCase(
        name = "Multiple block history zero ratio sum -> Simple Average",
        feeHistory = FeeHistory(
          oldestBlock = 100uL,
          baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
          reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
          gasUsedRatio = listOf(0, 0, 0, 0).map { it.toDouble() },
          baseFeePerBlobGas = listOf(200, 210, 220, 230, 240).map { it.toULong() },
          blobGasUsedRatio = listOf(0, 0, 0, 0).map { it.toDouble() }
        ),
        weightedAverageBaseFee = 115.0,
        weightedAveragePriorityFee = 1150.0,
        weightedAverageBlobBaseFee = 215.0
      ),
      FeeHistoryTestCase(
        name = "Empty history",
        feeHistory = FeeHistory(
          oldestBlock = 100uL,
          baseFeePerGas = emptyList(),
          reward = emptyList(),
          gasUsedRatio = emptyList(),
          baseFeePerBlobGas = emptyList(),
          blobGasUsedRatio = emptyList()
        ),
        weightedAverageBaseFee = 0.0,
        weightedAveragePriorityFee = 0.0,
        weightedAverageBlobBaseFee = 0.0
      )
    )

    @JvmStatic
    fun averageWeightedFeesCalculatorTestCases(): Stream<Arguments> {
      return feeHistoryTestCases.map {
        Arguments.of(it.name, it.weightedAverageBaseFee, it.feeHistory)
      }.stream()
    }

    @JvmStatic
    fun averageWeightedBaseFeesCalculatorTestCases(): Stream<Arguments> {
      return feeHistoryTestCases.map {
        Arguments.of(it.name, it.weightedAverageBaseFee, it.feeHistory)
      }.stream()
    }

    @JvmStatic
    fun averageWeightedPriorityFeesCalculatorTestCases(): Stream<Arguments> {
      return feeHistoryTestCases.map {
        Arguments.of(it.name, it.weightedAveragePriorityFee, it.feeHistory)
      }.stream()
    }

    @JvmStatic
    fun averageWeightedBlobBaseFeesCalculatorTestCases(): Stream<Arguments> {
      return feeHistoryTestCases.map {
        Arguments.of(it.name, it.weightedAverageBlobBaseFee, it.feeHistory)
      }.stream()
    }
  }
}
