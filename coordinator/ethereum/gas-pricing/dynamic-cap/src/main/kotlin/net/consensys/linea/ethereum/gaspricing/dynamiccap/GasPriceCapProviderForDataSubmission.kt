package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.gas.GasPriceCaps
import linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

class GasPriceCapProviderForDataSubmission(
  private val config: Config,
  private val gasPriceCapProvider: GasPriceCapProvider,
  metricsFacade: MetricsFacade,
) : GasPriceCapProvider {
  data class Config(
    val maxPriorityFeePerGasCap: ULong,
    val maxFeePerGasCap: ULong,
    val maxFeePerBlobGasCap: ULong,
  )
  private var lastGasPriceCap: AtomicReference<GasPriceCaps?> = AtomicReference(null)

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "l1.blobsubmission.maxpriorityfeepergascap",
      description = "Max priority fee per gas cap for blob submission on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxPriorityFeePerGasCap?.toLong() ?: 0 },
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "l1.blobsubmission.maxfeepergascap",
      description = "Max fee per gas cap for blob submission on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxFeePerGasCap?.toLong() ?: 0 },
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "l1.blobsubmission.maxfeeperblobgascap",
      description = "Max fee per blob gas cap for blob submission on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxFeePerBlobGasCap?.toLong() ?: 0 },
    )
  }

  private fun ULong.coerceWithFallback(cap: ULong): ULong = coerceAtMost(cap).let { if (it <= 0uL) cap else it }

  private fun coerceGasPriceCaps(gasPriceCaps: GasPriceCaps): GasPriceCaps {
    val maxPriorityFeePerGasCap = gasPriceCaps.maxPriorityFeePerGasCap
      .coerceWithFallback(config.maxPriorityFeePerGasCap)

    val maxFeePerGasCap = (
      gasPriceCaps.maxBaseFeePerGasCap?.let { it + maxPriorityFeePerGasCap }
        ?: gasPriceCaps.maxFeePerGasCap
      )
      .coerceWithFallback(config.maxFeePerGasCap)

    val maxFeePerBlobGasCap = gasPriceCaps.maxFeePerBlobGasCap
      .coerceWithFallback(config.maxFeePerBlobGasCap)

    return GasPriceCaps(
      maxFeePerGasCap = maxFeePerGasCap,
      maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
      maxFeePerBlobGasCap = maxFeePerBlobGasCap,
    )
  }

  override fun getGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return gasPriceCapProvider.getGasPriceCaps(targetL2BlockNumber)
      .thenApply { it?.run(this::coerceGasPriceCaps) }
      .thenPeek {
        this.lastGasPriceCap.set(it)
      }
  }

  override fun getGasPriceCapsWithCoefficient(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return gasPriceCapProvider.getGasPriceCapsWithCoefficient(targetL2BlockNumber)
      .thenApply { it?.run(this::coerceGasPriceCaps) }
      .thenPeek {
        this.lastGasPriceCap.set(it)
      }
  }
}
