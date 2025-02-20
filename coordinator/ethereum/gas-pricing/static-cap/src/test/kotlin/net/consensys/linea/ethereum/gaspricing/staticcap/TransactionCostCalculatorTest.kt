package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.math.abs

class TransactionCostCalculatorTest {
  private val config = TransactionCostCalculator.Config(1.0, 30000000U)
  private val oldCalculatorConfig = GasUsageRatioWeightedAverageFeesCalculator.Config(
    // We may want to price L2 gas cheaper than L1 gas
    baseFeeCoefficient = 0.02,
    priorityFeeCoefficient = 0.02,
    baseFeeBlobCoefficient = 0.02,
    blobSubmissionExpectedExecutionGas = 69000,
    expectedBlobGas = 131_000
  )
  private val legacyFeesCalculator = GasUsageRatioWeightedAverageFeesCalculator(oldCalculatorConfig)

  private val variableFeesCalculatorConfig = VariableFeesCalculator.Config(
    margin = 1.2,
    bytesPerDataSubmission = 131_000u,
    blobSubmissionExpectedExecutionGas = 69000u,
    expectedBlobGas = 131_000u
  )
  private val variableFeesCalculator = VariableFeesCalculator(variableFeesCalculatorConfig)
  private val transactionCostCalculator = TransactionCostCalculator(variableFeesCalculator, config)

  private val gWei = 1000000000UL

  @Test
  fun transactionCostCalculatorIsLessSusceptibleToGasSpikes() {
    val regularL1Fees = FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(gWei),
      reward = listOf(listOf(1000UL)), // not a big impact
      gasUsedRatio = listOf(0.25),
      baseFeePerBlobGas = listOf(100UL),
      blobGasUsedRatio = listOf(0.25)
    )

    val legacyGasPriceUnderRegularConditions = legacyFeesCalculator.calculateFees(regularL1Fees)
    val transactionCostUnderRegularConditions = transactionCostCalculator.calculateFees(regularL1Fees)

    // Under regular conditions calculators are comparable
    assertThat(differenceInPercentage(legacyGasPriceUnderRegularConditions, transactionCostUnderRegularConditions))
      .isLessThan(1.0)

    val blobGasSpikeL1Fees =
      regularL1Fees.copy(baseFeePerBlobGas = regularL1Fees.baseFeePerBlobGas.map { 1000UL * gWei })
    val legacyGasPriceDuringGasSpike = legacyFeesCalculator.calculateFees(blobGasSpikeL1Fees)
    val transactionCostDuringGasSpike = transactionCostCalculator.calculateFees(blobGasSpikeL1Fees)

    // But during gas spike legacy gas calculator becomes less stable
    assertThat(legacyGasPriceDuringGasSpike / transactionCostDuringGasSpike).isGreaterThan(2.0)
  }

  private fun differenceInPercentage(a: Double, b: Double) = abs(a - b) / b
}
