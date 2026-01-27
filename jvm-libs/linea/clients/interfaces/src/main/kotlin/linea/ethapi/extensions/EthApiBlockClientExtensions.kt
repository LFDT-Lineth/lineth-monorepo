package linea.ethapi.extensions

import linea.domain.BlockParameter
import linea.ethapi.EthApiBlockClient
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun EthApiBlockClient.getBlockParameterNumber(blockParameter: BlockParameter): SafeFuture<ULong> {
  return if (blockParameter is BlockParameter.BlockNumber) {
    SafeFuture.completedFuture(blockParameter.getNumber())
  } else if (blockParameter == BlockParameter.Tag.EARLIEST) {
    SafeFuture.completedFuture(0UL)
  } else {
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
