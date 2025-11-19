package linea.ethapi

import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface EthApiBlockClient {
  fun ethBlockNumber(): SafeFuture<ULong>
  fun ethFindBlockByNumberFullTxs(blockParameter: BlockParameter): SafeFuture<Block?>
  fun ethGetBlockByNumberFullTxs(blockParameter: BlockParameter): SafeFuture<Block> {
    return ethFindBlockByNumberFullTxs(blockParameter).thenApply { block ->
      block ?: throw IllegalArgumentException("block=$blockParameter not found!")
    }
  }
  fun ethFindBlockByNumberTxHashes(blockParameter: BlockParameter): SafeFuture<BlockWithTxHashes?>
  fun ethGetBlockByNumberTxHashes(blockParameter: BlockParameter): SafeFuture<BlockWithTxHashes> {
    return ethFindBlockByNumberTxHashes(blockParameter).thenApply { block ->
      block ?: throw IllegalArgumentException("block=$blockParameter not found!")
    }
  }
  // fun ethGetBlockByHashFullTxs(blockHash: ByteArray): SafeFuture<Block?>
  // fun ethGetBlockByHashTxHashes(blockHash: ByteArray): SafeFuture<BlockWithTxHashes?>
}
