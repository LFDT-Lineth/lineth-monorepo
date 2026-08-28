package lineth.ethereum.gaspricing.dynamiccap

import lineth.gaspricing.GasPriceCapProvider
import net.consensys.linea.metrics.MetricsFacade

class GasPriceCapProviderForFinalization(
  config: Config,
  gasPriceCapProvider: GasPriceCapProvider,
  metricsFacade: MetricsFacade,
) : GasPriceCapProvider by GasPriceCapProviderWithHardCaps(
  config = GasPriceCapProviderWithHardCaps.Config(
    maxPriorityFeePerGasCap = config.maxPriorityFeePerGasCap,
    maxFeePerGasCap = config.maxFeePerGasCap,
    maxFeePerBlobGasCap = null,
    capMultiplier = config.gasPriceCapMultiplier,
    metricsPrefix = "l1.finalization",
  ),
  delegate = gasPriceCapProvider,
  metricsFacade = metricsFacade,
) {
  data class Config(
    val maxPriorityFeePerGasCap: ULong,
    val maxFeePerGasCap: ULong,
    val gasPriceCapMultiplier: Double = 1.0,
  )
}
