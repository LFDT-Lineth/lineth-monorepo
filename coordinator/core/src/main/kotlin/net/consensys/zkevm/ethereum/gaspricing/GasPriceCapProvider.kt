package net.consensys.zkevm.ethereum.gaspricing

import linea.domain.gas.GasPriceCaps
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface GasPriceCapProvider {
  fun getGasPriceCaps(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?>
  fun getGasPriceCapsWithCoefficient(targetL2BlockNumber: Long): SafeFuture<GasPriceCaps?>
}
