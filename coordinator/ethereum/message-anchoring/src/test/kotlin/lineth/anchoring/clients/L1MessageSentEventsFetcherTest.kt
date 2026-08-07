package lineth.anchoring.clients

import lineth.EthLogsSearcher
import lineth.SearchDirection
import lineth.contract.events.createL1MessageSentV1Logs
import lineth.domain.BlockParameter
import lineth.domain.EthLog
import lineth.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

class L1MessageSentEventsFetcherTest {
  @Test
  fun `returns an empty list when rolling search finds no message events`() {
    val eventLogs = createL1MessageSentV1Logs(
      contractAddress = CONTRACT_ADDRESS,
      messageNumber = 1UL,
      messageHash = "01".decodeHex(),
      rollingHash = "02".decodeHex(),
    )
    val searcher = EmptyRollingSearchEthLogsSearcher(eventLogs.l1RollingHashUpdated.log)
    val fetcher = L1MessageSentEventsFetcher(
      l1SmartContractAddress = CONTRACT_ADDRESS,
      l1EventsSearcher = searcher,
      l1HighestBlock = BlockParameter.Tag.FINALIZED,
      l1EventSearchMaxBlockRange = 100U,
    )

    val events = fetcher.findL1MessageSentEvents(
      startingMessageNumber = 1UL,
      targetMessagesToFetch = 10U,
      fetchTimeout = 1.seconds,
      blockChunkSize = 100U,
    ).get()

    assertThat(events).isEmpty()
  }

  private class EmptyRollingSearchEthLogsSearcher(
    private val latestRollingHashLog: EthLog,
  ) : EthLogsSearcher {
    override fun findLog(
      fromBlock: BlockParameter,
      toBlock: BlockParameter,
      chunkSize: Int,
      address: String,
      topics: List<String>,
      shallContinueToSearch: (EthLog) -> SearchDirection?,
    ): SafeFuture<EthLog?> = SafeFuture.completedFuture(latestRollingHashLog)

    override fun getLogsRollingForward(
      fromBlock: BlockParameter,
      toBlock: BlockParameter,
      address: String,
      topics: List<String?>,
      chunkSize: UInt,
      searchTimeout: Duration,
      stopAfterTargetLogsCount: UInt?,
    ): SafeFuture<EthLogsSearcher.LogSearchResult> =
      SafeFuture.completedFuture(
        EthLogsSearcher.LogSearchResult(
          logs = emptyList(),
          startBlockNumber = 100UL,
          endBlockNumber = 200UL,
        ),
      )

    override fun getLogs(
      fromBlock: BlockParameter,
      toBlock: BlockParameter,
      address: String,
      topics: List<String?>,
    ): SafeFuture<List<EthLog>> = SafeFuture.completedFuture(emptyList())
  }

  private companion object {
    const val CONTRACT_ADDRESS = "0x1111111111111111111111111111111111111111"
  }
}
