package net.consensys.linea.ethereum.gaspricing.staticcap

import linea.domain.FeeHistory
import net.consensys.linea.ethereum.gaspricing.FeesCalculator
import net.consensys.linea.ethereum.gaspricing.L2CalldataSizeAccumulator
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class L2CalldataBasedVariableFeesCalculatorTest {
  private val config = L2CalldataBasedVariableFeesCalculator.Config(
    feeChangeDenominator = 32u,
    calldataSizeBlockCount = 5u,
    maxBlockCalldataSize = 109000u,
  )
  private val feeHistory = FeeHistory(
    oldestBlock = 100uL,
    baseFeePerGas = listOf(100UL),
    reward = listOf(listOf(1000UL)),
    gasUsedRatio = listOf(0.25),
    baseFeePerBlobGas = listOf(100UL),
    blobGasUsedRatio = listOf(0.25),
  )
  private val variableFee = 15000.0
  val mockVariableFeesCalculator = mock<FeesCalculator> {
    on { calculateFees(eq(feeHistory)) } doReturn variableFee
  }

  @Test
  fun test_calculateFees_past_blocks_calldata_at_max_target() {
    val sumOfCalldataSize = (109000 * 5).toBigInteger() // maxBlockCalldataSize * calldataSizeBlockCount
    val mockl2CalldataSizeAccumulator = mock<L2CalldataSizeAccumulator> {
      on { getSumOfL2CalldataSize() } doReturn SafeFuture.completedFuture(sumOfCalldataSize)
    }
    // delta would be 1.0
    val delta = 1.0
    val expectedVariableFees = 15000.0 * (1.0 + delta / 32.0)

    val feesCalculator = L2CalldataBasedVariableFeesCalculator(
      config = config,
      variableFeesCalculator = mockVariableFeesCalculator,
      l2CalldataSizeAccumulator = mockl2CalldataSizeAccumulator,
    )

    // call calculateFees first to instantiate the lastVariableCost
    feesCalculator.calculateFees(feeHistory)

    assertThat(feesCalculator.calculateFees(feeHistory))
      .isEqualTo(expectedVariableFees)
  }

  @Test
  fun test_calculateFees_past_blocks_calldata_exceed_max_target() {
    // This could happen as the calldata from L2CalldataSizeAccumulator is just approximation
    val sumOfCalldataSize = (200000 * 5).toBigInteger()
    val mockl2CalldataSizeAccumulator = mock<L2CalldataSizeAccumulator> {
      on { getSumOfL2CalldataSize() } doReturn SafeFuture.completedFuture(sumOfCalldataSize)
    }
    // delta would be 1.0
    val delta = 1.0
    val expectedVariableFees = 15000.0 * (1.0 + delta / 32.0)

    val feesCalculator = L2CalldataBasedVariableFeesCalculator(
      config = config,
      variableFeesCalculator = mockVariableFeesCalculator,
      l2CalldataSizeAccumulator = mockl2CalldataSizeAccumulator,
    )

    // call calculateFees first to instantiate the lastVariableCost
    feesCalculator.calculateFees(feeHistory)

    assertThat(feesCalculator.calculateFees(feeHistory))
      .isEqualTo(expectedVariableFees)
  }

  @Test
  fun test_calculateFees_past_blocks_calldata_size_at_zero() {
    val mockl2CalldataSizeAccumulator = mock<L2CalldataSizeAccumulator> {
      on { getSumOfL2CalldataSize() } doReturn SafeFuture.completedFuture(BigInteger.ZERO)
    }

    val feesCalculator = L2CalldataBasedVariableFeesCalculator(
      config = config,
      variableFeesCalculator = mockVariableFeesCalculator,
      l2CalldataSizeAccumulator = mockl2CalldataSizeAccumulator,
    )

    // call calculateFees first to instantiate the lastVariableCost
    feesCalculator.calculateFees(feeHistory)

    assertThat(feesCalculator.calculateFees(feeHistory))
      .isEqualTo(15000.0)
  }

  @Test
  fun test_calculateFees_past_blocks_calldata_above_half_max() {
    val sumOfCalldataSize = (81750 * 5).toBigInteger()
    val mockl2CalldataSizeAccumulator = mock<L2CalldataSizeAccumulator> {
      on { getSumOfL2CalldataSize() } doReturn SafeFuture.completedFuture(sumOfCalldataSize)
    }
    // delta would be 0.5
    val delta = 0.5
    val expectedVariableFees = 15000.0 * (1.0 + delta / 32.0)

    val feesCalculator = L2CalldataBasedVariableFeesCalculator(
      config = config,
      variableFeesCalculator = mockVariableFeesCalculator,
      l2CalldataSizeAccumulator = mockl2CalldataSizeAccumulator,
    )

    // call calculateFees first to instantiate the lastVariableCost
    feesCalculator.calculateFees(feeHistory)

    assertThat(feesCalculator.calculateFees(feeHistory))
      .isEqualTo(expectedVariableFees)
  }

  @Test
  fun test_calculateFees_past_blocks_calldata_below_half_max() {
    val sumOfCalldataSize = (27250 * 5).toBigInteger()
    val mockl2CalldataSizeAccumulator = mock<L2CalldataSizeAccumulator> {
      on { getSumOfL2CalldataSize() } doReturn SafeFuture.completedFuture(sumOfCalldataSize)
    }

    val feesCalculator = L2CalldataBasedVariableFeesCalculator(
      config = config,
      variableFeesCalculator = mockVariableFeesCalculator,
      l2CalldataSizeAccumulator = mockl2CalldataSizeAccumulator,
    )

    // call calculateFees first to instantiate the lastVariableCost
    feesCalculator.calculateFees(feeHistory)

    assertThat(feesCalculator.calculateFees(feeHistory))
      .isEqualTo(15000.0)
  }

  @Test
  fun test_calculateFees_increase_to_more_than_double_when_past_blocks_calldata_at_max_target() {
    val sumOfCalldataSize = (109000 * 5).toBigInteger() // maxBlockCalldataSize * calldataSizeBlockCount
    val mockl2CalldataSizeAccumulator = mock<L2CalldataSizeAccumulator> {
      on { getSumOfL2CalldataSize() } doReturn SafeFuture.completedFuture(sumOfCalldataSize)
    }

    val feesCalculator = L2CalldataBasedVariableFeesCalculator(
      config = config,
      variableFeesCalculator = mockVariableFeesCalculator,
      l2CalldataSizeAccumulator = mockl2CalldataSizeAccumulator,
    )

    // call calculateFees first to instantiate the lastVariableCost
    feesCalculator.calculateFees(feeHistory)

    // With (1 + 1/32) as the rate, after 23 calls
    // the expectedVariableFees should be increased to more than double
    var calculatedFee = 0.0
    (0..22).forEach { _ ->
      calculatedFee = feesCalculator.calculateFees(feeHistory)
    }

    assertThat(calculatedFee).isGreaterThan(15000.0 * 2.0)
  }

  @Test
  fun test_calculateFees_decrease_to_less_than_half_when_past_blocks_calldata_at_zero() {
    val mockVariableFeesCalculator = mock<FeesCalculator>()
    whenever(mockVariableFeesCalculator.calculateFees(eq(feeHistory)))
      .thenReturn(variableFee, 0.0)

    val mockl2CalldataSizeAccumulator = mock<L2CalldataSizeAccumulator> {
      on { getSumOfL2CalldataSize() } doReturn SafeFuture.completedFuture(BigInteger.ZERO)
    }

    val feesCalculator = L2CalldataBasedVariableFeesCalculator(
      config = config,
      variableFeesCalculator = mockVariableFeesCalculator,
      l2CalldataSizeAccumulator = mockl2CalldataSizeAccumulator,
    )

    // call calculateFees first to instantiate the lastVariableCost
    feesCalculator.calculateFees(feeHistory)

    // With (1 - 1/32) as the rate, after 22 calls
    // the expectedVariableFees should be decreased to less than half
    var calculatedFee = 0.0
    (0..21).forEach { _ ->
      calculatedFee = feesCalculator.calculateFees(feeHistory)
    }

    assertThat(calculatedFee).isLessThan(15000.0 / 2.0)
  }

  @Test
  fun test_calculateFees_when_block_count_is_zero() {
    val sumOfCalldataSize = (109000 * 5).toBigInteger() // maxBlockCalldataSize * calldataSizeBlockCount
    val mockl2CalldataSizeAccumulator = mock<L2CalldataSizeAccumulator> {
      on { getSumOfL2CalldataSize() } doReturn SafeFuture.completedFuture(sumOfCalldataSize)
    }

    val feesCalculator = L2CalldataBasedVariableFeesCalculator(
      config = L2CalldataBasedVariableFeesCalculator.Config(
        feeChangeDenominator = 32u,
        calldataSizeBlockCount = 0u, // set zero to disable calldata-based variable fees
        maxBlockCalldataSize = 109000u,
      ),
      variableFeesCalculator = mockVariableFeesCalculator,
      l2CalldataSizeAccumulator = mockl2CalldataSizeAccumulator,
    )

    // call calculateFees first to instantiate the lastVariableCost
    feesCalculator.calculateFees(feeHistory)

    // The returned variable fees should always be 15000.0
    // as calldata-based variable fees is disabled
    assertThat(feesCalculator.calculateFees(feeHistory))
      .isEqualTo(15000.0)
  }
}
