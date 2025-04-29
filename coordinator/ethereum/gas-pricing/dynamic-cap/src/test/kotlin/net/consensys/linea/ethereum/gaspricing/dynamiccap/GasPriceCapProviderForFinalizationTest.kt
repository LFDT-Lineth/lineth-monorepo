package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.gas.GasPriceCaps
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito.RETURNS_DEEP_STUBS
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture

class GasPriceCapProviderForFinalizationTest {
  private val returnRawGasPriceCaps = GasPriceCaps(
    maxBaseFeePerGasCap = 7000000000uL, // 7GWei
    maxPriorityFeePerGasCap = 1000000000uL, // 1GWei
    maxFeePerGasCap = 8000000000uL, // 8GWei
    maxFeePerBlobGasCap = 500000000uL // 0.5GWei
  )
  private val returnMultipliedRawGasPriceCaps = GasPriceCaps(
    maxBaseFeePerGasCap = (returnRawGasPriceCaps.maxBaseFeePerGasCap!!.toDouble() * 0.9).toULong(), // 6.3GWei
    maxPriorityFeePerGasCap = (returnRawGasPriceCaps.maxPriorityFeePerGasCap.toDouble() * 0.9).toULong(), // 0.9GWei
    maxFeePerGasCap = 7200000000uL, // 7.2GWei
    maxFeePerBlobGasCap = (returnRawGasPriceCaps.maxFeePerBlobGasCap.toDouble() * 0.9).toULong() // 0.45GWei
  )
  private val maxPriorityFeePerGasCap = 3000000000uL // 3GWei
  private val maxFeePerGasCap = 10000000000uL // 10GWei
  private val gasPriceCapMultiplier = 2.0
  private val expectedMaxPriorityFeePerGasCap =
    (maxPriorityFeePerGasCap.toDouble() * gasPriceCapMultiplier).toULong()
  private val expectedMaxFeePerGasCap =
    (maxFeePerGasCap.toDouble() * gasPriceCapMultiplier).toULong()

  private lateinit var gasPriceCapProviderForFinalization: GasPriceCapProvider
  private lateinit var mockedGasPriceCapProvider: GasPriceCapProvider
  private lateinit var mockedMetricsFacade: MetricsFacade

  private fun createGasPriceCapProviderForFinalization(
    returnRawGasPriceCaps: GasPriceCaps,
    config: GasPriceCapProviderForFinalization.Config = GasPriceCapProviderForFinalization.Config(
      maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
      maxFeePerGasCap = maxFeePerGasCap,
      gasPriceCapMultiplier = gasPriceCapMultiplier
    )
  ): GasPriceCapProviderForFinalization {
    mockedGasPriceCapProvider = mock<GasPriceCapProvider> {
      on { getGasPriceCaps(any()) } doReturn SafeFuture.completedFuture(returnRawGasPriceCaps)
      on { getGasPriceCapsWithCoefficient(any()) } doReturn
        SafeFuture.completedFuture(returnMultipliedRawGasPriceCaps)
    }
    mockedMetricsFacade = mock<MetricsFacade>(defaultAnswer = RETURNS_DEEP_STUBS)
    return GasPriceCapProviderForFinalization(
      config = config,
      gasPriceCapProvider = mockedGasPriceCapProvider,
      metricsFacade = mockedMetricsFacade
    )
  }

  @BeforeEach
  fun beforeEach() {
    gasPriceCapProviderForFinalization = createGasPriceCapProviderForFinalization(returnRawGasPriceCaps)
  }

  @Test
  fun `gas price caps of finalization should be returned correctly`() {
    assertThat(
      gasPriceCapProviderForFinalization.getGasPriceCaps(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = returnRawGasPriceCaps.maxPriorityFeePerGasCap,
        maxFeePerGasCap = returnRawGasPriceCaps.maxFeePerGasCap,
        maxFeePerBlobGasCap = 0uL
      )
    )
  }

  @Test
  fun `gas price caps with coefficient of finalization should be returned correctly`() {
    assertThat(
      gasPriceCapProviderForFinalization.getGasPriceCapsWithCoefficient(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = returnMultipliedRawGasPriceCaps.maxPriorityFeePerGasCap,
        maxFeePerGasCap = returnMultipliedRawGasPriceCaps.maxFeePerGasCap,
        maxFeePerBlobGasCap = 0uL
      )
    )
  }

  @Test
  fun `gas price caps of finalization should be capped correctly`() {
    gasPriceCapProviderForFinalization = createGasPriceCapProviderForFinalization(
      returnRawGasPriceCaps = GasPriceCaps(
        maxBaseFeePerGasCap = 22000000000uL, // 22GWei (exceeds 20GWei upper bound)
        maxPriorityFeePerGasCap = 8000000000uL, // 8GWei (exceeds 6GWei upper bound)
        maxFeePerGasCap = 30000000000uL, // 30GWei (exceeds 20GWei upper bound)
        maxFeePerBlobGasCap = 8000000000uL // 8GWei (ignorable)
      )
    )
    assertThat(
      gasPriceCapProviderForFinalization.getGasPriceCaps(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = expectedMaxPriorityFeePerGasCap, // 6GWei (capped at 6GWei upper bound)
        maxFeePerGasCap = expectedMaxFeePerGasCap, // 20GWei (capped at 20GWei upper bound)
        maxFeePerBlobGasCap = 0uL
      )
    )

    gasPriceCapProviderForFinalization = createGasPriceCapProviderForFinalization(
      returnRawGasPriceCaps = GasPriceCaps(
        maxBaseFeePerGasCap = 21000000000uL, // 21GWei (exceeds 20GWei upper bound)
        maxPriorityFeePerGasCap = 4000000000uL,
        maxFeePerGasCap = 25000000000uL, // 25GWei (exceeds 20GWei upper bound)
        maxFeePerBlobGasCap = 8000000000uL // 8GWei (ignorable)
      )
    )
    assertThat(
      gasPriceCapProviderForFinalization.getGasPriceCaps(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = 4000000000uL,
        maxFeePerGasCap = expectedMaxFeePerGasCap, // 20GWei (capped at 20GWei upper bound)
        maxFeePerBlobGasCap = 0uL
      )
    )

    gasPriceCapProviderForFinalization = createGasPriceCapProviderForFinalization(
      returnRawGasPriceCaps = GasPriceCaps(
        maxBaseFeePerGasCap = 13000000000uL, // 13GWei
        maxPriorityFeePerGasCap = 10000000000uL, // 10GWei (exceeds 6GWei upper bound)
        maxFeePerGasCap = 23000000000uL, // 26GWei (exceeds 20GWei upper bound)
        maxFeePerBlobGasCap = 8000000000uL // 8GWei (ignorable)
      )
    )
    assertThat(
      gasPriceCapProviderForFinalization.getGasPriceCaps(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = expectedMaxPriorityFeePerGasCap, // 6GWei (capped at 6GWei upper bound)
        maxFeePerGasCap = 19000000000uL,
        maxFeePerBlobGasCap = 0uL
      )
    )
  }
}
