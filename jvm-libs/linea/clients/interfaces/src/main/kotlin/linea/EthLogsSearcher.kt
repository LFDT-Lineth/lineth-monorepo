package linea

import net.consensys.linea.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class SearchDirection {
  FORWARD,
  BACKWARD
}

interface EthLogsSearcher {
  /**
   * Shall search for the Log until shallContinueToSearchPredicate returns null.
   * if fromBlock..toBlock range is too large, it shall break into smaller chunks
   * and perform a binary search;
   */
  fun findLog(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    chunkSize: Int = 1000,
    address: String,
    topics: List<String>,
    shallContinueToSearch: (build.linea.domain.EthLog) -> SearchDirection? // null means stop searching
  ): SafeFuture<build.linea.domain.EthLog?>

  fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>
  ): SafeFuture<List<build.linea.domain.EthLog>>
}
