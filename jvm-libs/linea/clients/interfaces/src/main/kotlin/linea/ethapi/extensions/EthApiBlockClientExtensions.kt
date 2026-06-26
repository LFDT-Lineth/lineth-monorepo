package linea.ethapi.extensions

import linea.domain.BlockParameter
import linea.ethapi.EthApiBlockClient
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun EthApiBlockClient.getBlockParameterNumber(blockParameter: BlockParameter): SafeFuture<ULong> {
  return when (blockParameter) {
    is BlockParameter.BlockNumber -> SafeFuture.completedFuture(blockParameter.getNumber())
    BlockParameter.Tag.EARLIEST -> SafeFuture.completedFuture(0UL)
    is BlockParameter.BlockHash ->
      throw UnsupportedOperationException(
        "Block hash resolution requires ethGetBlockByHash; blockParameter=$blockParameter",
      )
    else ->
      this.ethGetBlockByNumberTxHashes(blockParameter)
        .thenApply { block -> block.number }
  }
}

fun EthApiBlockClient.getAbsoluteBlockNumbers(
  fromBlock: BlockParameter,
  toBlock: BlockParameter,
): SafeFuture<Pair<ULong, ULong>> {
  return SafeFuture.collectAll(
    getBlockParameterNumber(fromBlock),
    getBlockParameterNumber(toBlock),
  ).thenApply { (start, end) ->
    start to end
  }
}
