package linea.ethapi

import io.vertx.core.Vertx
import linea.EthLogsSearcher
import linea.domain.BlockParameter
import linea.domain.EthLog
import linea.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.seconds

class LogsSearcherTest {
  private val testAddress = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01"
  private val testTopic1 = "0x1111111111111111111111111111111111111111111111111111111111111111"
  private val testTopic2 = "0x2222222222222222222222222222222222222222222222222222222222222222"
  private val templateLog = EthLog(
    blockNumber = 100UL,
    address = testAddress.decodeHex(),
    topics = emptyList(),
    data = "0x1234567890abcdef".decodeHex(),
    transactionHash = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef".decodeHex(),
    transactionIndex = 0UL,
    logIndex = 0UL,
    blockHash = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef".decodeHex(),
    removed = false,
  )
  private val initialLogs = listOf(
    templateLog.copy(
      blockNumber = 200UL,
      topics = listOf(testTopic1.decodeHex(), testTopic2.decodeHex()),
    ),
    templateLog.copy(
      blockNumber = 300UL,
      topics = listOf(testTopic1.decodeHex()),
    ),
    templateLog.copy(
      blockNumber = 350UL,
      topics = listOf(testTopic2.decodeHex()),
    ),
    templateLog.copy(
      blockNumber = 400UL,
      address = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa02".decodeHex(),
      topics = listOf(testTopic2.decodeHex()),
    ),
  )
  private lateinit var vertx: Vertx
  private lateinit var fakeElClient: FakeEthApiClient
  private lateinit var searcher: EthLogsSearcher

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx()
    fakeElClient = FakeEthApiClient(initialLogs.toSet())
    fakeElClient.setLatestBlockTag(initialLogs.last().blockNumber + 1UL)
    searcher = EthLogsSearcherImpl(vertx, fakeElClient)
  }

  @AfterEach
  fun tearDown() {
    vertx.close()
  }

  @Test
  fun `should return logs within block range`() {
    fakeElClient.setFinalizedBlockTag(450UL)
    val result = searcher.getLogsRollingForward(
      fromBlock = BlockParameter.BlockNumber(10UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      address = testAddress,
      topics = emptyList(),
      chunkSize = 10U,
      searchTimeout = 1000000.seconds,
      stopAfterTargetLogsCount = null,
    ).get()

    assertThat(result.logs).isEqualTo(initialLogs.take(3))
    assertThat(result.startBlockNumber).isEqualTo(10UL)
    assertThat(result.endBlockNumber).isEqualTo(450UL)
  }

  @Test
  fun `should return logs within block range in face of temporary errors`() {
    fakeElClient.setFinalizedBlockTag(450UL)
    // The search should retry on errors when find logs (all will throw 5 times)
    // and not to skip retrieving any of the event logs
    fakeElClient.setGetLogsBlocksForcedErrorsCounts(
      mutableMapOf(
        1UL to 5U, // will throw error 5 times when find logs in range 1..100
        101UL to 5U, // will throw error 5 times when find logs in range 101..200
        201UL to 5U, // will throw error 5 times when find logs in range 201..300
        301UL to 5U, // will throw error 5 times when find logs in range 301..400
      ),
    )
    val result = searcher.getLogsRollingForward(
      fromBlock = BlockParameter.BlockNumber(1UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      address = testAddress,
      topics = emptyList(),
      chunkSize = 100U,
      searchTimeout = 1000000.seconds,
      stopAfterTargetLogsCount = null,
    ).get()

    assertThat(result.logs).isEqualTo(initialLogs.take(3))
    assertThat(result.startBlockNumber).isEqualTo(1UL)
    assertThat(result.endBlockNumber).isEqualTo(450UL)
  }

  @Test
  fun `should throw exception when the search timeout reached in face of consistent errors`() {
    fakeElClient.setFinalizedBlockTag(450UL)
    fakeElClient.setGetLogsBlocksForcedErrorsCounts(
      mutableMapOf(
        401UL to null, // will always throw error when find logs in range 301..450
      ),
    )
    assertThatThrownBy {
      searcher.getLogsRollingForward(
        fromBlock = BlockParameter.BlockNumber(1UL),
        toBlock = BlockParameter.Tag.FINALIZED,
        address = testAddress,
        topics = emptyList(),
        chunkSize = 100U,
        searchTimeout = 2.seconds,
        stopAfterTargetLogsCount = null,
      ).get()
    }.hasCauseInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("for error testing when calling getLogs")
  }

  @Test
  fun `should filter logs by address`() {
    val result = searcher.getLogsRollingForward(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.BlockNumber(500UL),
      address = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa02",
      topics = emptyList(),
      chunkSize = 100U,
      searchTimeout = 5.seconds,
      stopAfterTargetLogsCount = null,
    ).get()

    assertThat(result.logs).isEqualTo(listOf(initialLogs[3]))
  }

  @Test
  fun `should filter logs by topics`() {
    val result = searcher.getLogsRollingForward(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.BlockNumber(400UL),
      address = testAddress,
      topics = listOf(testTopic1),
      chunkSize = 100U,
      searchTimeout = 5.seconds,
      stopAfterTargetLogsCount = null,
    ).get()

    assertThat(result.logs).isEqualTo(initialLogs.take(2))
  }

  @Test
  fun `should respect stopAfterTargetLogsCount`() {
    val result = searcher.getLogsRollingForward(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.BlockNumber(3000UL),
      address = testAddress,
      topics = emptyList(),
      chunkSize = 50U,
      searchTimeout = 2.hours,
      stopAfterTargetLogsCount = 1U,
    ).get()

    assertThat(result.logs).isEqualTo(initialLogs.take(1))
  }

  @Test
  fun `should respect searchTimeout`() {
    searcher = EthLogsSearcherImpl(
      vertx,
      fakeElClient,
      config = EthLogsSearcherImpl.Config(loopSuccessBackoffDelay = 1.seconds),
    )
    val result = searcher.getLogsRollingForward(
      fromBlock = BlockParameter.BlockNumber(initialLogs.first().blockNumber),
      toBlock = BlockParameter.BlockNumber(3000UL),
      address = testAddress,
      topics = emptyList(),
      chunkSize = 50U,
      searchTimeout = 1.seconds, // it only has time for the first iteration
      stopAfterTargetLogsCount = null,
    ).get()

    assertThat(result.logs).hasSize(1)
  }

  @Test
  fun `should return empty list when no logs match`() {
    val result = searcher.getLogsRollingForward(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.BlockNumber(300UL),
      address = testAddress,
      topics = listOf("0x3333333333333333333333333333333333333333333333333333333333333333"),
      chunkSize = 100U,
      searchTimeout = 5.seconds,
      stopAfterTargetLogsCount = null,
    ).get()

    assertThat(result.logs).isEmpty()
  }
}
