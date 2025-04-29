package net.consensys.linea.ethereum.gaspricing.dynamiccap

import io.vertx.junit5.VertxExtension
import linea.domain.gas.GasPriceCaps
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito.RETURNS_DEEP_STUBS
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture

@ExtendWith(VertxExtension::class)
class GasPriceCapProviderForDataSubmissionTest {
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
  private val expectedMaxPriorityFeePerGasCap = 3000000000uL // 3GWei
  private val expectedMaxFeePerGasCap = 10000000000uL // 10GWei
  private val expectedMaxFeePerBlobGasCap = 700000000uL // 0.7GWei

  private lateinit var gasPriceCapProviderForDataSubmission: GasPriceCapProvider
  private lateinit var mockedGasPriceCapProvider: GasPriceCapProvider
  private lateinit var mockedMetricsFacade: MetricsFacade

  private fun createGasPriceCapProviderForDataSubmission(
    returnRawGasPriceCaps: GasPriceCaps,
    config: GasPriceCapProviderForDataSubmission.Config = GasPriceCapProviderForDataSubmission.Config(
      maxPriorityFeePerGasCap = expectedMaxPriorityFeePerGasCap,
      maxFeePerGasCap = expectedMaxFeePerGasCap,
      maxFeePerBlobGasCap = expectedMaxFeePerBlobGasCap
    )
  ): GasPriceCapProviderForDataSubmission {
    mockedGasPriceCapProvider = mock<GasPriceCapProvider> {
      on { getGasPriceCaps(any()) } doReturn SafeFuture.completedFuture(returnRawGasPriceCaps)
      on { getGasPriceCapsWithCoefficient(any()) } doReturn
        SafeFuture.completedFuture(returnMultipliedRawGasPriceCaps)
    }
    mockedMetricsFacade = mock<MetricsFacade>(defaultAnswer = RETURNS_DEEP_STUBS)
    return GasPriceCapProviderForDataSubmission(
      config = config,
      gasPriceCapProvider = mockedGasPriceCapProvider,
      metricsFacade = mockedMetricsFacade
    )
  }

  @BeforeEach
  fun beforeEach() {
    gasPriceCapProviderForDataSubmission = createGasPriceCapProviderForDataSubmission(returnRawGasPriceCaps)
  }

  @Test
  fun `gas price caps of data submission should be returned correctly`() {
    assertThat(
      gasPriceCapProviderForDataSubmission.getGasPriceCaps(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = 1000000000uL, // 1GWei
        maxFeePerGasCap = 8000000000uL, // 8GWei
        maxFeePerBlobGasCap = 500000000uL // 0.5GWei
      )
    )
  }

  @Test
  fun `gas price caps with coefficient of data submission should be returned correctly`() {
    assertThat(
      gasPriceCapProviderForDataSubmission.getGasPriceCapsWithCoefficient(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = returnMultipliedRawGasPriceCaps.maxPriorityFeePerGasCap,
        maxFeePerGasCap = returnMultipliedRawGasPriceCaps.maxFeePerGasCap,
        maxFeePerBlobGasCap = returnMultipliedRawGasPriceCaps.maxFeePerBlobGasCap
      )
    )
  }

  @Test
  fun `gas price caps of data submission should be capped correctly`() {
    gasPriceCapProviderForDataSubmission = createGasPriceCapProviderForDataSubmission(
      returnRawGasPriceCaps = GasPriceCaps(
        maxBaseFeePerGasCap = 9000000000uL, // 9GWei
        maxPriorityFeePerGasCap = 8000000000uL, // 8GWei (exceeds 3GWei upper bound)
        maxFeePerGasCap = 17000000000uL, // 17GWei (exceeds 10GWei upper bound)
        maxFeePerBlobGasCap = 8000000000uL // 8GWei (exceeds 0.7GWei upper bound)
      )
    )
    assertThat(
      gasPriceCapProviderForDataSubmission.getGasPriceCaps(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = expectedMaxPriorityFeePerGasCap, // 3GWei (capped at 3GWei upper bound)
        maxFeePerGasCap = expectedMaxFeePerGasCap, // 10GWei (capped at 10GWei upper bound)
        maxFeePerBlobGasCap = expectedMaxFeePerBlobGasCap // 0.7GWei (capped at 0.7GWei upper bound)
      )
    )

    gasPriceCapProviderForDataSubmission = createGasPriceCapProviderForDataSubmission(
      returnRawGasPriceCaps = GasPriceCaps(
        maxBaseFeePerGasCap = 5000000000uL, // 5GWei
        maxPriorityFeePerGasCap = 8000000000uL, // 8GWei (exceeds 3GWei upper bound)
        maxFeePerGasCap = 13000000000uL, // 13GWei (exceeds 10GWei upper bound)
        maxFeePerBlobGasCap = 8000000000uL // 8GWei (exceeds 0.7GWei upper bound)
      )
    )
    assertThat(
      gasPriceCapProviderForDataSubmission.getGasPriceCaps(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = expectedMaxPriorityFeePerGasCap, // 3GWei (capped at 3GWei upper bound)
        maxFeePerGasCap = 8000000000uL, // 8GWei
        maxFeePerBlobGasCap = expectedMaxFeePerBlobGasCap // 0.7GWei (capped at 0.7GWei upper bound)
      )
    )

    gasPriceCapProviderForDataSubmission = createGasPriceCapProviderForDataSubmission(
      returnRawGasPriceCaps = GasPriceCaps(
        maxBaseFeePerGasCap = 15000000000uL, // 15GWei (exceeds 10GWei upper bound)
        maxPriorityFeePerGasCap = 8000000000uL, // 8GWei (exceeds 3GWei upper bound)
        maxFeePerGasCap = 23000000000uL, // 23GWei (exceeds 10GWei upper bound)
        maxFeePerBlobGasCap = 8000000000uL // 8GWei (exceeds 0.7GWei upper bound)
      )
    )
    assertThat(
      gasPriceCapProviderForDataSubmission.getGasPriceCaps(100L).get()
    ).isEqualTo(
      GasPriceCaps(
        maxPriorityFeePerGasCap = expectedMaxPriorityFeePerGasCap, // 3GWei (capped at 3GWei upper bound)
        maxFeePerGasCap = expectedMaxFeePerGasCap, // 10GWei (capped at 10GWei upper bound)
        maxFeePerBlobGasCap = expectedMaxFeePerBlobGasCap // 0.7GWei (capped at 0.7GWei upper bound)
      )
    )
  }
}
