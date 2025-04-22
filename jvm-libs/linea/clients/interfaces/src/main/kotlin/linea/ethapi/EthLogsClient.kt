package linea.ethapi

import linea.domain.BlockParameter
import linea.domain.EthLog
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface EthLogsClient {
  fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>
  ): SafeFuture<List<EthLog>>
}
