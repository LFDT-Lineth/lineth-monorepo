package linea.contract.l1

import io.vertx.core.Vertx
import linea.contract.events.FactoryFinalizedStateUpdatedEvent
import linea.domain.BlockParameter
import linea.domain.toBlockParameter
import linea.ethapi.EthLogsSearcherImpl
import linea.ethapi.FakeEthApiClient
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import org.web3j.protocol.Web3j

class Web3JLinethRollupSmartContractClientReadOnlyTest {
  private val contractAddress = "0x" + "aa".repeat(20)
  private lateinit var vertx: Vertx
  private lateinit var l1Client: FakeEthApiClient
  private lateinit var client: Web3JLinethRollupSmartContractClientReadOnly

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx()
    l1Client = FakeEthApiClient()
    client = Web3JLinethRollupSmartContractClientReadOnly(
      web3j = mock<Web3j>(), // not used by findFinalizedStateEvent
      contractAddress = contractAddress,
      ethLogsSearcher = EthLogsSearcherImpl(vertx = vertx, ethApiClient = l1Client),
      // deliberately small so the search spans multiple bounded chunks instead of one query
      l1EventSearchMaxBlockRange = 100u,
    )
  }

  @AfterEach
  fun tearDown() {
    vertx.close()
  }

  @Test
  fun `findFinalizedStateEvent locates the event by finalized block number across bounded chunks`() {
    // FinalizedStateUpdated events are sparse across the L1 range; the indexed L2 block number is
    // monotonic with the L1 block, so the binary search must converge on the matching one.
    l1Client.setLogs(
      listOf(
        event(l1Block = 1_000UL, l2FinalizedBlockNumber = 100UL, forcedTransactionNumber = 5UL),
        event(l1Block = 3_000UL, l2FinalizedBlockNumber = 200UL, forcedTransactionNumber = 9UL),
        event(l1Block = 5_000UL, l2FinalizedBlockNumber = 300UL, forcedTransactionNumber = 14UL),
      ),
    )
    l1Client.setLatestBlockTag(6_000UL)

    val result = client.findFinalizedStateEvent(
      upToBlock = BlockParameter.Tag.LATEST,
      finalizedBlockNumber = 200UL,
    ).get()

    assertThat(result).isNotNull
    assertThat(result!!.blockNumber).isEqualTo(200UL)
    assertThat(result.forcedTransactionNumber).isEqualTo(9UL)
  }

  @Test
  fun `findFinalizedStateEvent returns null when no event matches the finalized block number`() {
    l1Client.setLogs(
      listOf(
        event(l1Block = 1_000UL, l2FinalizedBlockNumber = 100UL, forcedTransactionNumber = 5UL),
      ),
    )
    l1Client.setLatestBlockTag(6_000UL)

    val result = client.findFinalizedStateEvent(
      upToBlock = BlockParameter.Tag.LATEST,
      finalizedBlockNumber = 999UL,
    ).get()

    assertThat(result).isNull()
  }

  @Test
  fun `findFinalizedStateEvent falls back to a full-range search when the cached window overshoots`() {
    l1Client.setLogs(
      listOf(
        event(l1Block = 1_000UL, l2FinalizedBlockNumber = 100UL, forcedTransactionNumber = 5UL),
        event(l1Block = 3_000UL, l2FinalizedBlockNumber = 200UL, forcedTransactionNumber = 9UL),
        event(l1Block = 5_000UL, l2FinalizedBlockNumber = 300UL, forcedTransactionNumber = 14UL),
      ),
    )
    l1Client.setLatestBlockTag(6_000UL)

    // Advance the high-water-mark to the latest finalization (L1 block 5_000).
    assertThat(client.findFinalizedStateEvent(BlockParameter.Tag.LATEST, finalizedBlockNumber = 300UL).get())
      .isNotNull

    // A subsequent query for an earlier finalization sits before the cached window, so the bounded
    // search misses and the full-range fallback must still resolve it.
    val result = client.findFinalizedStateEvent(BlockParameter.Tag.LATEST, finalizedBlockNumber = 200UL).get()

    assertThat(result).isNotNull
    assertThat(result!!.blockNumber).isEqualTo(200UL)
    assertThat(result.forcedTransactionNumber).isEqualTo(9UL)
  }

  @Test
  fun `findFinalizedStateEvent recovers when the cached window sits after a historical upToBlock`() {
    l1Client.setLogs(
      listOf(
        event(l1Block = 1_000UL, l2FinalizedBlockNumber = 100UL, forcedTransactionNumber = 5UL),
        event(l1Block = 3_000UL, l2FinalizedBlockNumber = 200UL, forcedTransactionNumber = 9UL),
        event(l1Block = 5_000UL, l2FinalizedBlockNumber = 300UL, forcedTransactionNumber = 14UL),
      ),
    )
    l1Client.setLatestBlockTag(6_000UL)

    // Advance the high-water-mark to L1 block 5_000.
    assertThat(client.findFinalizedStateEvent(BlockParameter.Tag.LATEST, finalizedBlockNumber = 300UL).get())
      .isNotNull

    // Query a historical upToBlock (2_000) that is before the cached window, so the bounded search
    // would be an invalid range; it must recover via the full-range fallback.
    val result = client.findFinalizedStateEvent(
      upToBlock = 2_000UL.toBlockParameter(),
      finalizedBlockNumber = 100UL,
    ).get()

    assertThat(result).isNotNull
    assertThat(result!!.blockNumber).isEqualTo(100UL)
    assertThat(result.forcedTransactionNumber).isEqualTo(5UL)
  }

  private fun event(l1Block: ULong, l2FinalizedBlockNumber: ULong, forcedTransactionNumber: ULong) =
    FactoryFinalizedStateUpdatedEvent.createEthLog(
      blockNumber = l1Block,
      contractAddress = contractAddress,
      l2FinalizedBlockNumber = l2FinalizedBlockNumber,
      forcedTransactionNumber = forcedTransactionNumber,
    )
}
