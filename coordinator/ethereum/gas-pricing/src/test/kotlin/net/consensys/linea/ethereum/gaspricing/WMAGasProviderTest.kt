package net.consensys.linea.ethereum.gaspricing

import linea.domain.FeeHistory
import linea.kotlin.toULong
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture

class WMAGasProviderTest {
  private val feeHistory =
    FeeHistory(
      oldestBlock = 100uL,
      baseFeePerGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      reward = listOf(1000, 1100, 1200, 1300).map { listOf(it.toULong()) },
      gasUsedRatio = listOf(0.25, 0.5, 0.75, 0.9),
      baseFeePerBlobGas = listOf(100, 110, 120, 130, 140).map { it.toULong() },
      blobGasUsedRatio = listOf(0.25, 0.5, 0.75, 0.9),
    )
  private val chainId = 999
  private val gasLimit = 100000uL
  private val mockFeesFetcher =
    mock<FeesFetcher> {
      on { getL1EthGasPriceData() } doReturn SafeFuture.completedFuture(feeHistory)
    }
  private val l1PriorityFeeCalculator: FeesCalculator =
    WMAFeesCalculator(
      WMAFeesCalculator.Config(
        baseFeeCoefficient = 0.0,
        priorityFeeWmaCoefficient = 1.0,
      ),
    )

  @Test
  fun `getMaxFeePerGas should return correct value based on baseFeePerGas and WMA of priority fee`() {
    val maxFeePerGasCap = 1000000000uL
    val maxPriorityFeePerGasCap = 100000000uL
    val maxFeePerBlobGasCap = 1000000000uL
    val wmaGasProvider =
      WMAGasProvider(
        chainId.toLong(),
        mockFeesFetcher,
        l1PriorityFeeCalculator,
        WMAGasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = maxFeePerBlobGasCap,
        ),
      )

    // WMA = (140*0.0) + ((1000*0.25*1 + 1100*0.5*2 + 1200*0.75*3 + 1300*0.9*4) / (0.25*1 + 0.5*2 + 0.75*3 + 0.9*4)) * 1.0 = 1229.57746
    // MaxFeePerGas = min(140*2 + 1229.57746, 100000000) = 1509.57
    val calculatedMaxFeePerGas = wmaGasProvider.getMaxFeePerGas().toULong()
    assertThat(calculatedMaxFeePerGas).isEqualTo(1509uL) // 1509.57 rounded down
  }

  @Test
  fun `getMaxPriorityFeePerGas should return correct WMA of priority fee`() {
    val maxFeePerGasCap = 1000000000uL
    val maxPriorityFeePerGasCap = 100000000uL
    val maxFeePerBlobGasCap = 1000000000uL
    val wmaGasProvider =
      WMAGasProvider(
        chainId.toLong(),
        mockFeesFetcher,
        l1PriorityFeeCalculator,
        WMAGasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = maxFeePerBlobGasCap,
        ),
      )

    val calculatedMaxPriorityFeePerGas = wmaGasProvider.getMaxPriorityFeePerGas().toULong()
    assertThat(calculatedMaxPriorityFeePerGas).isEqualTo(1229uL) // 1229.57 rounded down
  }

  @Test
  fun `getMaxFeePerGas should return 1000 if calculated max fee per gas is larger than 1000`() {
    val maxFeePerGasCap = 1000uL
    val maxPriorityFeePerGasCap = 1000uL
    val maxFeePerBlobGasCap = 100uL
    val wmaGasProvider =
      WMAGasProvider(
        chainId.toLong(),
        mockFeesFetcher,
        l1PriorityFeeCalculator,
        WMAGasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = maxFeePerBlobGasCap,
        ),
      )

    val calculatedMaxFeePerGas = wmaGasProvider.getMaxFeePerGas().toULong()
    assertThat(calculatedMaxFeePerGas).isEqualTo(maxFeePerGasCap)
  }

  @Test
  fun `getMaxPriorityFeePerGas should return 800 if WMA of priority fee is larger than 800`() {
    val maxFeePerGasCap = 1000uL
    val maxPriorityFeePerGasCap = 800uL
    val maxFeePerBlobGasCap = 1000000000uL
    val wmaGasProvider =
      WMAGasProvider(
        chainId.toLong(),
        mockFeesFetcher,
        l1PriorityFeeCalculator,
        WMAGasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = maxFeePerBlobGasCap,
        ),
      )

    val calculatedMaxPriorityFeePerGas = wmaGasProvider.getMaxPriorityFeePerGas().toULong()
    assertThat(calculatedMaxPriorityFeePerGas).isEqualTo(maxPriorityFeePerGasCap)
  }

  @Test
  fun `getEIP1559GasFees should return correct maxPriorityFeePerGas and maxFeePerGas`() {
    val maxFeePerGasCap = 1000000000uL
    val maxPriorityFeePerGasCap = 100000000uL
    val maxFeePerBlobGasCap = 1000000000uL
    val wmaGasProvider =
      WMAGasProvider(
        chainId.toLong(),
        mockFeesFetcher,
        l1PriorityFeeCalculator,
        WMAGasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = maxFeePerBlobGasCap,
        ),
      )

    val calculatedFees = wmaGasProvider.getEIP1559GasFees()
    assertThat(calculatedFees.maxPriorityFeePerGas).isEqualTo(1229uL) // 1229.57 rounded down
    assertThat(calculatedFees.maxFeePerGas).isEqualTo(1509uL) // 1509.57 rounded down
  }

  @Test
  fun `getEIP1559GasFees should return both maxPriorityFeePerGas and maxFeePerGas as 1000 if exceeds max cap`() {
    val maxFeePerGasCap = 1000uL
    val maxPriorityFeePerGasCap = 100000000uL
    val maxFeePerBlobGasCap = 1000000000uL
    val wmaGasProvider =
      WMAGasProvider(
        chainId.toLong(),
        mockFeesFetcher,
        l1PriorityFeeCalculator,
        WMAGasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = maxFeePerBlobGasCap,
        ),
      )

    val calculatedFees = wmaGasProvider.getEIP1559GasFees()
    assertThat(calculatedFees.maxPriorityFeePerGas).isEqualTo(maxFeePerGasCap)
    assertThat(calculatedFees.maxFeePerGas).isEqualTo(maxFeePerGasCap)
  }

  @Test
  fun `getEIP4844GasFees should return correct maxPriorityFeePerGas and maxFeePerGas and maxFeePerBlobGas`() {
    val maxFeePerGasCap = 1000000000uL
    val maxPriorityFeePerGasCap = 100000000uL
    val maxFeePerBlobGasCap = 1000000000uL
    val wmaGasProvider =
      WMAGasProvider(
        chainId.toLong(),
        mockFeesFetcher,
        l1PriorityFeeCalculator,
        WMAGasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = maxFeePerBlobGasCap,
        ),
      )

    val calculatedFees = wmaGasProvider.getEIP4844GasFees()
    assertThat(calculatedFees.eip1559GasFees.maxPriorityFeePerGas).isEqualTo(1229uL) // 1229.57 rounded down
    assertThat(calculatedFees.eip1559GasFees.maxFeePerGas).isEqualTo(1509uL) // 1509.57 rounded down
    assertThat(calculatedFees.maxFeePerBlobGas).isEqualTo(maxFeePerBlobGasCap)
  }

  @Test
  fun `getEIP4844GasFees should return both calculatedFees as 100 if exceeds max cap`() {
    val maxFeePerGasCap = 100uL
    val maxPriorityFeePerGasCap = 100uL
    val maxFeePerBlobGasCap = 100uL
    val wmaGasProvider =
      WMAGasProvider(
        chainId.toLong(),
        mockFeesFetcher,
        l1PriorityFeeCalculator,
        WMAGasProvider.Config(
          gasLimit = gasLimit,
          maxFeePerGasCap = maxFeePerGasCap,
          maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
          maxFeePerBlobGasCap = maxFeePerBlobGasCap,
        ),
      )

    val calculatedFees = wmaGasProvider.getEIP4844GasFees()
    assertThat(calculatedFees.eip1559GasFees.maxPriorityFeePerGas).isEqualTo(maxFeePerGasCap)
    assertThat(calculatedFees.eip1559GasFees.maxFeePerGas).isEqualTo(maxFeePerGasCap)
    assertThat(calculatedFees.maxFeePerBlobGas).isEqualTo(maxFeePerBlobGasCap)
  }
}
