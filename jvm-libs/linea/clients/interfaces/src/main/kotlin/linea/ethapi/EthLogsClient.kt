package linea.ethapi

import linea.domain.BlockParameter
import linea.domain.EthLog
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class FilterOptions(
  val fromBlock: BlockParameter,
  val toBlock: BlockParameter,
  val address: String,
  val topics: List<String?>,
)

interface EthLogsClient {
  fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>,
  ): SafeFuture<List<EthLog>>

  fun ethGetLogs(filterOptions: FilterOptions): SafeFuture<List<EthLog>> = getLogs(
    filterOptions.fromBlock,
    filterOptions.toBlock,
    filterOptions.address,
    filterOptions.topics,
  )
}
