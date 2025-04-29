package linea.domain.gas

import linea.kotlin.toGWei

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
