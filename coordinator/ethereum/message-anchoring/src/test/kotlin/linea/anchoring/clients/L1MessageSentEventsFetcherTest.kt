package linea.anchoring.clients

import linea.EthLogsSearcher
import linea.contract.events.createL1MessageSentV1Logs
import linea.domain.BlockParameter
import linea.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.seconds

class L1MessageSentEventsFetcherTest {
  @Test
  fun `returns an empty list when rolling search finds no message events`() {
    val searcher = mock<EthLogsSearcher>()
    val eventLogs = createL1MessageSentV1Logs(
      contractAddress = CONTRACT_ADDRESS,
      messageNumber = 1UL,
      messageHash = "01".decodeHex(),
      rollingHash = "02".decodeHex(),
    )
    whenever(searcher.findLog(any(), any(), any(), any(), any(), any()))
      .thenReturn(SafeFuture.completedFuture(eventLogs.l1RollingHashUpdated.log))
    whenever(searcher.getLogsRollingForward(any(), any(), any(), any(), any(), any(), anyOrNull()))
      .thenReturn(
        SafeFuture.completedFuture(
          EthLogsSearcher.LogSearchResult(
            logs = emptyList(),
            startBlockNumber = 100UL,
            endBlockNumber = 200UL,
          ),
        ),
      )
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

  private companion object {
    const val CONTRACT_ADDRESS = "0x1111111111111111111111111111111111111111"
  }
}
