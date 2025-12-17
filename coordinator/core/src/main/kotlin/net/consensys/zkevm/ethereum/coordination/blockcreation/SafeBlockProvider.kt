package net.consensys.zkevm.ethereum.coordination.blockcreation

import linea.domain.Block
import linea.domain.BlockHeaderSummary
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface SafeBlockProvider {
  fun getLatestSafeBlock(): SafeFuture<Block>

  fun getLatestSafeBlockHeader(): SafeFuture<BlockHeaderSummary> {
    return getLatestSafeBlock().thenApply { it.headerSummary }
  }
}
