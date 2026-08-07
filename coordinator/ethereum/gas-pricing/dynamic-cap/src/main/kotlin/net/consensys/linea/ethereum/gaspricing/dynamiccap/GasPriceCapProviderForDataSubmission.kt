package net.consensys.linea.ethereum.gaspricing.dynamiccap

import linea.gaspricing.GasPriceCapProvider
import lineth.metrics.MetricsFacade

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
