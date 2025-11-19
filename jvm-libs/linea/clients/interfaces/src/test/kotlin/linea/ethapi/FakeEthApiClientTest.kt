package linea.ethapi

import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.EthLog
import linea.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

class FakeEthApiClientTest {
  private lateinit var fakeEthApiClient: FakeEthApiClient
  private val testAddress = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01"
  private val testTopic1 = "0x1111111111111111111111111111111111111111111111111111111111111111"
  private val testTopic2 = "0x2222222222222222222222222222222222222222222222222222222222222222"
  private val templateLog = EthLog(
    blockNumber = 0UL,
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
      blockNumber = 100UL,
      topics = listOf(testTopic1.decodeHex(), testTopic2.decodeHex()),
    ),
    templateLog.copy(
      blockNumber = 200UL,
      topics = listOf(testTopic1.decodeHex()),
    ),
    templateLog.copy(
      blockNumber = 250UL,
      topics = listOf(testTopic2.decodeHex()),
    ),
    templateLog.copy(
      blockNumber = 300UL,
      address = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb01".decodeHex(),
      topics = listOf(testTopic2.decodeHex()),
    ),
  )

  @BeforeEach
  fun setUp() {
    fakeEthApiClient = FakeEthApiClient(initialLogs.toSet())
  }

  @Test
  fun `should return logs within block range`() {
    val logs = fakeEthApiClient.getLogs(
      fromBlock = 0UL.toBlockParameter(),
      toBlock = 300UL.toBlockParameter(),
      address = testAddress,
      emptyList(),
    ).get()

    assertThat(logs).isEqualTo(initialLogs.take(3))
  }

  @Test
  fun `should filter logs by address`() {
    val logs = fakeEthApiClient.getLogs(
      fromBlock = 0UL.toBlockParameter(),
      toBlock = 200UL.toBlockParameter(),
      address = testAddress,
      emptyList(),
    ).get()

    assertThat(logs).isEqualTo(initialLogs.take(2))
  }

  @Test
  fun `should filter logs by topics`() {
    fakeEthApiClient.getLogs(
      fromBlock = 0UL.toBlockParameter(),
      toBlock = 300UL.toBlockParameter(),
      address = testAddress,
      listOf(testTopic1),
    ).get()
      .also { logs ->
        assertThat(logs).isEqualTo(initialLogs.take(2))
      }

    fakeEthApiClient.getLogs(
      fromBlock = 0UL.toBlockParameter(),
      toBlock = 300UL.toBlockParameter(),
      address = testAddress,
      listOf(null, testTopic2),
    ).get()
      .also { logs ->
        assertThat(logs).isEqualTo(listOf(initialLogs[0]))
      }
  }

  @Test
  fun `should return empty list when no logs match`() {
    val logs = fakeEthApiClient.getLogs(
      fromBlock = 0UL.toBlockParameter(),
      toBlock = 300UL.toBlockParameter(),
      address = testAddress,
      listOf("0x3333333333333333333333333333333333333333333333333333333333333333"),
    ).get()

    assertThat(logs).isEmpty()
  }

  @Test
  fun `should handle null topics as wildcard`() {
    val logs = fakeEthApiClient.getLogs(
      fromBlock = 0UL.toBlockParameter(),
      toBlock = 300UL.toBlockParameter(),
      address = testAddress,
      listOf(null),
    ).get()

    assertThat(logs).isEqualTo(initialLogs.take(3))
  }

  @Test
  fun `should FINALIZED and adjust SAFE and LATEST automatically`() {
    FakeEthApiClient(
      initialTagsBlocks = mapOf(
        BlockParameter.Tag.EARLIEST to 0UL,
        BlockParameter.Tag.FINALIZED to 100UL,
        BlockParameter.Tag.SAFE to 110UL,
        BlockParameter.Tag.LATEST to 120UL,
        BlockParameter.Tag.PENDING to 121UL,
      ),
    ).also { client ->
      client.setFinalizedBlockTag(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).get().number).isEqualTo(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.SAFE).get().number).isEqualTo(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.FINALIZED).get().number).isEqualTo(500UL)

      client.setFinalizedBlockTag(20UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).get().number).isEqualTo(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.SAFE).get().number).isEqualTo(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.FINALIZED).get().number).isEqualTo(20UL)
    }
  }

  @Test
  fun `should update SAFE and adjust FINALIZED and LATEST automatically`() {
    FakeEthApiClient(
      initialTagsBlocks = mapOf(
        BlockParameter.Tag.EARLIEST to 0UL,
        BlockParameter.Tag.FINALIZED to 100UL,
        BlockParameter.Tag.SAFE to 110UL,
        BlockParameter.Tag.LATEST to 120UL,
        BlockParameter.Tag.PENDING to 121UL,
      ),
    ).also { client ->
      client.setSafeBlockTag(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).get().number).isEqualTo(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.SAFE).get().number).isEqualTo(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.FINALIZED).get().number).isEqualTo(100UL)

      // set SAFE back, should not affect FINALIZED ONLY
      client.setSafeBlockTag(20UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).get().number).isEqualTo(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.SAFE).get().number).isEqualTo(20UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.FINALIZED).get().number).isEqualTo(20UL)
    }
  }

  @Test
  fun `should update LATEST and adjust SAFE and FINALIZED automatically`() {
    FakeEthApiClient(
      initialTagsBlocks = mapOf(
        BlockParameter.Tag.EARLIEST to 0UL,
        BlockParameter.Tag.FINALIZED to 100UL,
        BlockParameter.Tag.SAFE to 110UL,
        BlockParameter.Tag.LATEST to 120UL,
        BlockParameter.Tag.PENDING to 121UL,
      ),
    ).also { client ->
      client.setLatestBlockTag(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).get().number).isEqualTo(500UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.SAFE).get().number).isEqualTo(110UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.FINALIZED).get().number).isEqualTo(100UL)

      // set Latest back, should not affect SAFE and FINALIZED
      client.setLatestBlockTag(20UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.LATEST).get().number).isEqualTo(20UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.SAFE).get().number).isEqualTo(20UL)
      assertThat(client.ethGetBlockByNumberFullTxs(BlockParameter.Tag.FINALIZED).get().number).isEqualTo(20UL)
    }
  }
}
