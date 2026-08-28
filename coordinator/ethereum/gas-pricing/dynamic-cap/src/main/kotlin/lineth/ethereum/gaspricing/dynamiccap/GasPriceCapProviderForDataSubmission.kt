package lineth.ethereum.gaspricing.dynamiccap

import lineth.gaspricing.GasPriceCapProvider
import net.consensys.linea.metrics.MetricsFacade

class GasPriceCapProviderForDataSubmission(
  config: Config,
  gasPriceCapProvider: GasPriceCapProvider,
  metricsFacade: MetricsFacade,
) : GasPriceCapProvider by GasPriceCapProviderWithHardCaps(
  config = GasPriceCapProviderWithHardCaps.Config(
    maxPriorityFeePerGasCap = config.maxPriorityFeePerGasCap,
    maxFeePerGasCap = config.maxFeePerGasCap,
    maxFeePerBlobGasCap = config.maxFeePerBlobGasCap,
    metricsPrefix = "l1.blobsubmission",
  ),
  delegate = gasPriceCapProvider,
  metricsFacade = metricsFacade,
) {
  data class Config(
    val maxPriorityFeePerGasCap: ULong,
    val maxFeePerGasCap: ULong,
    val maxFeePerBlobGasCap: ULong,
  )
}
