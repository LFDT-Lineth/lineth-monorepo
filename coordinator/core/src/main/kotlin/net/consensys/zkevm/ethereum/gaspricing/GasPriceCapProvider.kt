package net.consensys.zkevm.ethereum.gaspricing

import linea.kotlin.toGWei
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class GasPriceCaps(
  val maxPriorityFeePerGasCap: ULong,
  val maxFeePerGasCap: ULong,
  val maxFeePerBlobGasCap: ULong,
  val maxBaseFeePerGasCap: ULong? = null
) {
  override fun toString(): String {
    return "maxPriorityFeePerGasCap=${maxPriorityFeePerGasCap.toGWei()} GWei," +
      if (maxBaseFeePerGasCap != null) {
        " maxBaseFeePerGasCap=${maxBaseFeePerGasCap.toGWei()} GWei,"
      } else {
        ""
      } +
      " maxFeePerGasCap=${maxFeePerGasCap.toGWei()} GWei," +
      " maxFeePerBlobGasCap=${maxFeePerBlobGasCap.toGWei()} GWei"
  }
}

interface GasPriceCapProvider {
  fun getGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?>
  fun getGasPriceCapsWithCoefficient(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?>
}
