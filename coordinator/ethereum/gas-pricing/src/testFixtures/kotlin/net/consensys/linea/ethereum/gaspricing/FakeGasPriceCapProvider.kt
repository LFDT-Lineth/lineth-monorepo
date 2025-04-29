package net.consensys.linea.ethereum.gaspricing

import linea.domain.gas.GasPriceCaps
import net.consensys.zkevm.ethereum.gaspricing.GasPriceCapProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

val defaultGasPriceCaps = GasPriceCaps(
  maxPriorityFeePerGasCap = 10_000_000_000uL, // 10 gwei
  maxFeePerGasCap = 15_000_000_000uL,
  maxFeePerBlobGasCap = 10_00_000_000uL
)

class FakeGasPriceCapProvider(
  private val gasPriceCaps: GasPriceCaps = defaultGasPriceCaps
) : GasPriceCapProvider {
  override fun getGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return SafeFuture.completedFuture(gasPriceCaps)
  }

  override fun getGasPriceCapsWithCoefficient(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?> {
    return SafeFuture.completedFuture(gasPriceCaps)
  }
}
