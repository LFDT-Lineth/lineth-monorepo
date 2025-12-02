package net.consensys.zkevm.coordinator.blockcreation

import linea.domain.Block
import linea.domain.BlockParameter
import linea.ethapi.EthApiBlockClient
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

class GethCliqueSafeBlockProvider(
  private val ethApiBlockClient: EthApiBlockClient,
  private val config: Config,
) : SafeBlockProvider {
  data class Config(
    val blocksToFinalization: Long,
  )

  override fun getLatestSafeBlock(): SafeFuture<Block> {
    return ethApiBlockClient
      .ethGetBlockByNumberTxHashes(BlockParameter.Tag.LATEST)
      .thenCompose { block ->
        val safeBlockNumber = (block.number.toLong() - config.blocksToFinalization).coerceAtLeast(0)
        ethApiBlockClient.ethGetBlockByNumberFullTxs(BlockParameter.fromNumber(safeBlockNumber)).toSafeFuture()
      }
  }
}
