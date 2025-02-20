package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever

class VariableFeesCalculatorTest {

  private val config = VariableFeesCalculator.Config(
    margin = 1.1,
    bytesPerDataSubmission = 200_000u,
    blobSubmissionExpectedExecutionGas = 120_000u,
    expectedBlobGas = 131_000u
  )

  @Test
  fun test_calculateFees() {
    val mockFeeHistory = mock<FeeHistory>()
    whenever(mockFeeHistory.blocksRange()).thenReturn(1uL..100uL)
    val mockBaseFeeCalculator = mock<AverageWeightedFeesCalculator>()
    val averageWeightedBaseFees = 1.0
    whenever(mockBaseFeeCalculator.calculateFees(eq(mockFeeHistory))).thenReturn(averageWeightedBaseFees)
    val mockPriorityFeeCalculator = mock<AverageWeightedFeesCalculator>()
    val averageWeightedPriorityFees = 2.0
    whenever(mockPriorityFeeCalculator.calculateFees(eq(mockFeeHistory))).thenReturn(averageWeightedPriorityFees)
    val mockBlobBaseFeeCalculator = mock<AverageWeightedFeesCalculator>()
    val averageWeightedBlobBaseFees = 5.0
    whenever(mockBlobBaseFeeCalculator.calculateFees(eq(mockFeeHistory))).thenReturn(averageWeightedBlobBaseFees)

    // ((1+2)*120000 + 5*131000)*1.1/200000 = 5.5825
    val expectedVariableFees = (((1.0 + 2.0) * 120000.0 + 5.0 * 131000.0) * 1.1) / 200000.0

    val feesCalculator = VariableFeesCalculator(
      config = config,
      averageWeightedBaseFeesCalculator = mockBaseFeeCalculator,
      averageWeightedPriorityFeesCalculator = mockPriorityFeeCalculator,
      averageWeightedBlobBaseFeesCalculator = mockBlobBaseFeeCalculator
    )
    assertThat(feesCalculator.calculateFees(mockFeeHistory))
      .isEqualTo(expectedVariableFees)
  }
}
