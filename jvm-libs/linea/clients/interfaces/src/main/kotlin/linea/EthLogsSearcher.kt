package linea

import linea.domain.BlockInterval
import linea.domain.BlockParameter
import linea.domain.EthLog
import linea.ethapi.EthLogsClient
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

enum class SearchDirection {
  FORWARD,
  BACKWARD
}

interface EthLogsSearcher : EthLogsClient {
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
    shallContinueToSearch: (EthLog) -> SearchDirection? // null means stop searching
  ): SafeFuture<EthLog?>

  data class LogSearchResult(
    val logs: List<EthLog>,
    override val startBlockNumber: ULong,
    override val endBlockNumber: ULong
  ) : BlockInterval {
    val isEmpty: Boolean = logs.isEmpty()
  }

  /**
   * Fetches logs from L1 in chunks of chunkSize starting at fromBlock to toBlock.
   * It stops fetching logs when searchTimeout or stopAfterTargetLogsCount is reached.
   */
  fun getLogsRollingForward(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>,
    chunkSize: UInt = 1000u,
    searchTimeout: Duration,
    stopAfterTargetLogsCount: UInt? = null
  ): SafeFuture<LogSearchResult>
}
