package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.domain.gas.GasPriceCaps
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference

class GasPriceCapProviderForDataSubmission(
  private val config: Config,
  private val gasPriceCapProvider: GasPriceCapProvider,
  metricsFacade: MetricsFacade
) : GasPriceCapProvider {
  data class Config(
    val maxPriorityFeePerGasCap: ULong,
    val maxFeePerGasCap: ULong,
    val maxFeePerBlobGasCap: ULong
  )
  private var lastGasPriceCap: AtomicReference<GasPriceCaps?> = AtomicReference(null)

  init {
    require(config.maxPriorityFeePerGasCap >= 0uL) {
      "maxPriorityFeePerGasCap must be no less than 0. Value=${config.maxPriorityFeePerGasCap}"
    }

    require(config.maxFeePerGasCap >= 0uL) {
      "maxFeePerGasCap must be no less than 0. Value=${config.maxFeePerGasCap}"
    }

    require(config.maxFeePerBlobGasCap >= 0uL) {
      "maxFeePerBlobGasCap must be no less than 0. Value=${config.maxFeePerBlobGasCap}"
    }

    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "l1.blobsubmission.maxpriorityfeepergascap",
      description = "Max priority fee per gas cap for blob submission on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxPriorityFeePerGasCap?.toLong() ?: 0 }
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "l1.blobsubmission.maxfeepergascap",
      description = "Max fee per gas cap for blob submission on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxFeePerGasCap?.toLong() ?: 0 }
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.GAS_PRICE_CAP,
      name = "l1.blobsubmission.maxfeeperblobgascap",
      description = "Max fee per blob gas cap for blob submission on L1",
      measurementSupplier = { lastGasPriceCap.get()?.maxFeePerBlobGasCap?.toLong() ?: 0 }
    )
  }

  private fun coerceGasPriceCaps(gasPriceCaps: GasPriceCaps): GasPriceCaps {
    val maxPriorityFeePerGasCap = gasPriceCaps.maxPriorityFeePerGasCap
      .coerceAtMost(config.maxPriorityFeePerGasCap)
      .run { if (this <= 0uL) config.maxPriorityFeePerGasCap else this }

    val maxFeePerGasCap = (
      if (gasPriceCaps.maxBaseFeePerGasCap != null) {
        gasPriceCaps.maxBaseFeePerGasCap!! + maxPriorityFeePerGasCap
      } else {
        gasPriceCaps.maxFeePerGasCap
      }
      )
      .coerceAtMost(config.maxFeePerGasCap)
      .run { if (this <= 0uL) config.maxFeePerGasCap else this }

    val maxFeePerBlobGasCap = gasPriceCaps.maxFeePerBlobGasCap
      .coerceAtMost(config.maxFeePerBlobGasCap)
      .run { if (this <= 0uL) config.maxFeePerBlobGasCap else this }

    return GasPriceCaps(
      maxFeePerGasCap = maxFeePerGasCap,
      maxPriorityFeePerGasCap = maxPriorityFeePerGasCap,
      maxFeePerBlobGasCap = maxFeePerBlobGasCap
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
