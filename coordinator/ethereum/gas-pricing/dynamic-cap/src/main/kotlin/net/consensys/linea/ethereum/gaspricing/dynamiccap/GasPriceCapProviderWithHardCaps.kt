package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.gas.GasPriceCaps
import linea.gaspricing.GasPriceCapProvider
import linea.kotlin.toBigDecimal
import linea.kotlin.toULong
import linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

class GasPriceCapProviderWithHardCaps(
  private val config: Config,
  private val delegate: GasPriceCapProvider,
  metricsFacade: MetricsFacade,
) : GasPriceCapProvider {

  data class Config(
    val maxPriorityFeePerGasCap: ULong,
    val maxFeePerGasCap: ULong,
    // null means no blob transactions (e.g. finalization): blob cap is forced to 0
    val maxFeePerBlobGasCap: ULong?,
    val capMultiplier: Double = 1.0,
    val metricsPrefix: String,
  )

  private val lastGasPriceCap: AtomicReference<GasPriceCaps?> = AtomicReference(null)

  init {
    require(config.capMultiplier >= 0.0) {
      "capMultiplier must be no less than 0. Value=${config.capMultiplier}"
    }
    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "${config.metricsPrefix}.maxpriorityfeepergascap",
      description = "Max priority fee per gas cap for ${config.metricsPrefix} on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxPriorityFeePerGasCap?.toLong() ?: 0 },
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "${config.metricsPrefix}.maxfeepergascap",
      description = "Max fee per gas cap for ${config.metricsPrefix} on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxFeePerGasCap?.toLong() ?: 0 },
    )
    if (config.maxFeePerBlobGasCap != null) {
      metricsFacade.createGauge(
        category = LineaMetricsCategory.GAS_PRICE_CAP,
        name = "${config.metricsPrefix}.maxfeeperblobgascap",
        description = "Max fee per blob gas cap for ${config.metricsPrefix} on L1",
        measurementSupplier = { lastGasPriceCap.get()?.maxFeePerBlobGasCap?.toLong() ?: 0 },
      )
    }
  }

  // If the value after coercion is 0, fall back to the hard cap so transactions are never priced out.
  private fun ULong.coerceWithFallback(cap: ULong): ULong =
    coerceAtMost(cap).let { if (it == 0uL) cap else it }

  private fun effectiveCap(base: ULong): ULong =
    (base.toBigDecimal() * config.capMultiplier.toBigDecimal()).toBigInteger().toULong()

  private fun coerce(caps: GasPriceCaps): GasPriceCaps {
    val priorityCap = effectiveCap(config.maxPriorityFeePerGasCap)
    val feeCap = effectiveCap(config.maxFeePerGasCap)

    val priority = caps.maxPriorityFeePerGasCap.coerceWithFallback(priorityCap)
    val fee = (caps.maxBaseFeePerGasCap?.let { it + priority } ?: caps.maxFeePerGasCap)
      .coerceWithFallback(feeCap)
    val blobFee = config.maxFeePerBlobGasCap
      ?.let { hardCap -> caps.maxFeePerBlobGasCap.coerceWithFallback(hardCap) }
      ?: 0uL

    return GasPriceCaps(
      maxPriorityFeePerGasCap = priority,
      maxFeePerGasCap = fee,
      maxFeePerBlobGasCap = blobFee,
    )
  }

  private fun SafeFuture<GasPriceCaps?>.coerceAndTrack(): SafeFuture<GasPriceCaps?> =
    thenApply { it?.let(::coerce) }.thenPeek { lastGasPriceCap.set(it) }

  override fun getGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> =
    delegate.getGasPriceCaps(targetL2BlockNumber).coerceAndTrack()

  override fun getGasPriceCapsWithCoefficient(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> =
    delegate.getGasPriceCapsWithCoefficient(targetL2BlockNumber).coerceAndTrack()
}
