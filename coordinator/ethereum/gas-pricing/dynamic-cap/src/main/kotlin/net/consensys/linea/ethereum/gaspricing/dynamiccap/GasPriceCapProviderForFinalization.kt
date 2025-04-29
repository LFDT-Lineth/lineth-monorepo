package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.gas.GasPriceCaps
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

class GasPriceCapProviderForFinalization(
  private val config: Config,
  private val gasPriceCapProvider: GasPriceCapProvider,
  metricsFacade: MetricsFacade
) : GasPriceCapProvider {
  data class Config(
    val maxPriorityFeePerGasCap: ULong,
    val maxFeePerGasCap: ULong,
    val gasPriceCapMultiplier: Double = 1.0
  )
  private var lastGasPriceCap: AtomicReference<GasPriceCaps?> = AtomicReference(null)

  init {
    require(config.gasPriceCapMultiplier >= 0.0) {
      "gasPriceCapMultiplier must be no less than 0. Value=${config.gasPriceCapMultiplier}"
    }

    require(config.maxPriorityFeePerGasCap >= 0uL) {
      "maxPriorityFeePerGasCap must be no less than 0. Value=${config.maxPriorityFeePerGasCap}"
    }

    require(config.maxFeePerGasCap >= 0uL) {
      "maxFeePerGasCap must be no less than 0. Value=${config.maxFeePerGasCap}"
    }

    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "l1.finalization.maxpriorityfeepergascap",
      description = "Max priority fee per gas cap for finalization on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxPriorityFeePerGasCap?.toLong() ?: 0 }
    )

    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "l1.finalization.maxfeepergascap",
      description = "Max fee per gas cap for finalization on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxFeePerGasCap?.toLong() ?: 0 }
    )
  }

  private fun coerceGasPriceCaps(gasPriceCaps: GasPriceCaps): GasPriceCaps {
    val amplifiedMaxPriorityFeePerGasCap = (config.maxPriorityFeePerGasCap.toDouble() * config.gasPriceCapMultiplier)
      .toULong()
    val maxPriorityFeePerGasCap = gasPriceCaps.maxPriorityFeePerGasCap
      .coerceAtMost(amplifiedMaxPriorityFeePerGasCap)
      .run { if (this <= 0uL) amplifiedMaxPriorityFeePerGasCap else this }

    val amplifiedMaxFeePerGasCap = (config.maxFeePerGasCap.toDouble() * config.gasPriceCapMultiplier).toULong()
    val maxFeePerGasCap = (
      if (gasPriceCaps.maxBaseFeePerGasCap != null) {
        gasPriceCaps.maxBaseFeePerGasCap!! + maxPriorityFeePerGasCap
      } else {
        gasPriceCaps.maxFeePerGasCap
      }
      )
      .coerceAtMost(amplifiedMaxFeePerGasCap)
      .run { if (this <= 0uL) amplifiedMaxFeePerGasCap else this }

    return GasPriceCaps(
      maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
      maxFeePerGasCap = maxFeePerGasCap,
      maxFeePerBlobGasCap = 0uL
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
