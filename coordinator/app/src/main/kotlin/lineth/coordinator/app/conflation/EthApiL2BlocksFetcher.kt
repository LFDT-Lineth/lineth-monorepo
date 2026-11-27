package lineth.coordinator.app.conflation

import linea.domain.Block
import linea.domain.toBlockParameter
import linea.ethapi.EthApiClient
import lineth.coordination.riscv.rollup.L2BlocksFetcher
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Fetches L2 blocks with full transactions from the same L2 node conflation reads, so they encode exactly as the
 * conflated blocks did.
 */
class EthApiL2BlocksFetcher(
  private val l2EthClient: EthApiClient,
) : L2BlocksFetcher {
  override fun fetchBlocks(startBlockNumber: ULong, endBlockNumber: ULong): SafeFuture<List<Block>> {
    require(startBlockNumber <= endBlockNumber) { "invalid block range $startBlockNumber..$endBlockNumber" }
    return SafeFuture.collectAll(
      (startBlockNumber..endBlockNumber).map { l2EthClient.ethGetBlockByNumberFullTxs(it.toBlockParameter()) }.stream(),
    )
  }
}
