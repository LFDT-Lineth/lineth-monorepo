package net.consensys.linea.ethereum.gaspricing

import linea.domain.FeeHistory
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock

class BoundableFeeCalculatorTest {
  private val feeHistory =
    FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio =
      listOf(
        0.25,
        0.5,
        0.75,
        0.9,
      ),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio =
      listOf(
        0.25,
        0.5,
        0.75,
        0.9,
      ),
    )

  @Test
  fun calculateFees_fixedPriceAdded() {
    val gasPriceUpperBound = 9000000000000.0
    val gasPriceLowerBound = 1000000.0
    val gasPriceFixedCost = 17.0
    val calculatedL2GasPrice = gasPriceUpperBound - 100.0
    val expectedGasPrice = calculatedL2GasPrice + gasPriceFixedCost
    val mockFeesCalculator =
      mock<FeesCalculator> {
        on { calculateFees(eq(feeHistory)) } doReturn calculatedL2GasPrice
      }
    val boundableFeeCalculator =
      BoundableFeeCalculator(
        BoundableFeeCalculator.Config(gasPriceUpperBound, gasPriceLowerBound, gasPriceFixedCost),
        mockFeesCalculator,
      )
    Assertions.assertThat(boundableFeeCalculator.calculateFees(feeHistory)).isEqualTo(expectedGasPrice)
  }

  @Test
  fun calculateFess_upperBound() {
    val gasPriceUpperBound = 10000000000.0
    val gasPriceLowerBound = 1000000.0
    val gasPriceFixedCost = 100.0
    val calculatedL2GasPrice = gasPriceUpperBound - 50.0
    val expectedGasPrice = gasPriceUpperBound

    val mockFeesCalculator =
      mock<FeesCalculator> {
        on { calculateFees(any()) } doReturn calculatedL2GasPrice
      }
    val boundableFeeCalculator =
      BoundableFeeCalculator(
        BoundableFeeCalculator.Config(gasPriceUpperBound, gasPriceLowerBound, gasPriceFixedCost),
        mockFeesCalculator,
      )
    Assertions.assertThat(boundableFeeCalculator.calculateFees(feeHistory)).isEqualTo(expectedGasPrice)
  }

  @Test
  fun calculateFess_lowerBound() {
    val gasPriceUpperBound = 10000000000.0
    val gasPriceLowerBound = 1000000.0
    val gasPriceFixedCost = 100.0
    val calculatedL2GasPrice = gasPriceLowerBound.minus(300.0)
    val expectedGasPrice = gasPriceLowerBound

    val mockFeesCalculator =
      mock<FeesCalculator> {
        on { calculateFees(any()) } doReturn calculatedL2GasPrice
      }
    val boundableFeeCalculator =
      BoundableFeeCalculator(
        BoundableFeeCalculator.Config(gasPriceUpperBound, gasPriceLowerBound, gasPriceFixedCost),
        mockFeesCalculator,
      )
    Assertions.assertThat(boundableFeeCalculator.calculateFees(feeHistory)).isEqualTo(expectedGasPrice)
  }

  @Test
  fun constructor_throwsErrorIfNegativeParams() {
    val exception =
      assertThrows<IllegalArgumentException> {
        BoundableFeeCalculator(
          BoundableFeeCalculator.Config(
            -1000.0,
            -10000.0,
            -100.0,
          ),
          mock<FeesCalculator>(),
        )
      }
    Assertions.assertThat(exception)
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("feeUpperBound, feeLowerBound, and feeMargin must be no less than 0.")
  }

  @Test
  fun constructor_throwsErrorIfUpperSmallerThanLower() {
    val exception =
      assertThrows<IllegalArgumentException> {
        BoundableFeeCalculator(
          BoundableFeeCalculator.Config(
            1000.0,
            10000.0,
            10.0,
          ),
          mock<FeesCalculator>(),
        )
      }
    Assertions.assertThat(exception)
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("feeUpperBound must be no less than feeLowerBound.")
  }
}
