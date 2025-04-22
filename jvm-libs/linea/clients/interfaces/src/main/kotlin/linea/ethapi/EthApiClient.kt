package linea.ethapi

import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface EthApiClient : EthLogsClient {
  fun getBlockByNumber(
    blockParameter: BlockParameter
  ): SafeFuture<Block?>

  fun getBlockByNumberWithoutTransactionsData(
    blockParameter: BlockParameter
  ): SafeFuture<BlockWithTxHashes?>
}
