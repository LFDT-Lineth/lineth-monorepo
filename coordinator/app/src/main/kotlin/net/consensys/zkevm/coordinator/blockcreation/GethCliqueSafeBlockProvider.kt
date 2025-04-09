package net.consensys.zkevm.coordinator.blockcreation

import linea.domain.Block
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.web3j.domain.toWeb3j
import linea.web3j.toDomain
import net.consensys.linea.async.toSafeFuture
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameterName
import tech.pegasys.teku.infrastructure.async.SafeFuture

class GethCliqueSafeBlockProvider(
  private val web3j: Web3j,
  private val config: Config
) : SafeBlockProvider {
  data class Config(
    val blocksToFinalization: Long
  )

  override fun getLatestSafeBlock(): SafeFuture<Block> {
    return web3j
      .ethGetBlockByNumber(DefaultBlockParameterName.LATEST, false).sendAsync()
      .toSafeFuture()
      .thenCompose { block ->
        val safeBlockNumber = (block.block.number.toLong() - config.blocksToFinalization).coerceAtLeast(0)
        web3j.ethGetBlockByNumber(safeBlockNumber.toBlockParameter().toWeb3j(), true).sendAsync().toSafeFuture()
      }
      .thenApply { it.block.toDomain() }
  }
}
