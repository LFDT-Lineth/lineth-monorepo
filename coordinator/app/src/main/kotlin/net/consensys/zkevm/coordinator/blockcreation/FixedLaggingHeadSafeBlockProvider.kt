package net.consensys.zkevm.coordinator.blockcreation

import linea.domain.Block
import linea.domain.BlockParameter
import linea.ethapi.EthApiBlockClient
import linea.kotlin.minusCoercingUnderflow
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FixedLaggingHeadSafeBlockProvider(
  private val ethApiBlockClient: EthApiBlockClient,
  private val blocksToFinalization: ULong,
) : SafeBlockProvider {
  override fun getLatestSafeBlock(): SafeFuture<Block> {
    if (blocksToFinalization == 0UL) {
      return ethApiBlockClient.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).toSafeFuture()
    }

    return ethApiBlockClient
      .ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST)
      .thenCompose { block ->
        val safeBlockNumber = block.number.minusCoercingUnderflow(blocksToFinalization)
        ethApiBlockClient.ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(safeBlockNumber)).toSafeFuture()
      }
  }
}
