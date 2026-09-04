package lineth.gaspricing

import linea.domain.gas.GasPriceCaps
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

interface GasPriceCapProvider {
  fun getGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?>
  fun getGasPriceCapsWithCoefficient(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?>
}

interface GasPriceCapProviderV2 {
  fun getGasPriceCaps(timestamp: Instant): SafeFuture<GasPriceCaps?>
  fun getGasPriceCapsWithCoefficient(timestamp: Instant): SafeFuture<GasPriceCaps?>
}
