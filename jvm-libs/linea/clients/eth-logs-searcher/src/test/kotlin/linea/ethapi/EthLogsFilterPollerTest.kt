package linea.ethapi

import io.vertx.core.Vertx
import linea.domain.BlockParameter
import linea.domain.EthLog
import linea.ethapi.extensions.EthLogsFilterState
import linea.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class EthLogsFilterPollerTest {
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

  private lateinit var vertx: Vertx
  private lateinit var fakeEthApiClient: FakeEthApiClient
  private lateinit var poller: EthLogsFilterPoller

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx()
    fakeEthApiClient = FakeEthApiClient()
    fakeEthApiClient.setLatestBlockTag(100UL)
    fakeEthApiClient.setFinalizedBlockTag(50UL)
  }

  @AfterEach
  fun tearDown() {
    if (this::poller.isInitialized) {
      poller.stop().get()
    }
    vertx.close()
  }

  @Test
  fun `should throw exception when starting without consumer set`() {
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
    )

    assertThatThrownBy { poller.start().get() }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("Please setConsumer() before stating to poll for the logs")
  }

  @Test
  fun `should fetch and consume logs successfully`() {
    // Given: logs exist in the blockchain
    val logs = listOf(
      templateLog.copy(blockNumber = 10UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 20UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 30UL, logIndex = 0UL),
    )
    fakeEthApiClient.setLogs(logs)
    fakeEthApiClient.setFinalizedBlockTag(50UL)

    // When: poller starts and polls
    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()

    // Then: all logs should be consumed exactly once
    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactlyElementsOf(logs)
    }
  }

  @Test
  fun `should track progress and not reprocess logs after restart`() {
    // Given: initial logs
    val initialLogs = listOf(
      templateLog.copy(blockNumber = 10UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 20UL, logIndex = 0UL),
    )
    fakeEthApiClient.setLogs(initialLogs)
    fakeEthApiClient.setFinalizedBlockTag(30UL)

    // When: first poll processes initial logs
    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()

    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactlyElementsOf(initialLogs)
    }

    // And: finalized block advances and new logs are added in the new range
    val newLog = templateLog.copy(blockNumber = 40UL, logIndex = 0UL)
    fakeEthApiClient.addLogs(setOf(newLog))
    fakeEthApiClient.setFinalizedBlockTag(50UL)

    // Then: only new log should be processed (old logs not reprocessed)
    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactly(
        initialLogs[0],
        initialLogs[1],
        newLog,
      )
    }
  }

  @Test
  fun `should not reprocess logs when consumer is called multiple times`() {
    // Given: logs at same and different blocks
    val logs = listOf(
      templateLog.copy(blockNumber = 10UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 10UL, logIndex = 1UL),
      templateLog.copy(blockNumber = 20UL, logIndex = 0UL),
    )
    fakeEthApiClient.setLogs(logs)
    fakeEthApiClient.setFinalizedBlockTag(50UL)

    // When: poller processes logs
    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()

    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactlyElementsOf(logs)
    }

    // Wait a bit more to ensure no duplicates
    Thread.sleep(200)

    // Then: logs should be consumed exactly once (no duplicates)
    assertThat(consumedLogs).hasSize(logs.size)
    assertThat(consumedLogs).containsExactlyElementsOf(logs)
  }

  @Test
  fun `should retry failed log processing on next poll until successful and keep order`() {
    // Given: logs exist
    val logs = listOf(
      templateLog.copy(blockNumber = 10UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 20UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 30UL, logIndex = 0UL),
    )
    fakeEthApiClient.setLogs(logs)
    fakeEthApiClient.setFinalizedBlockTag(50UL)

    // When: consumer fails on second log, then succeeds
    val consumedLogs = mutableListOf<EthLog>()
    val throwErrorLatch = AtomicBoolean(true)
    val failureCount = AtomicInteger(0)

    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      pollingInterval = 100.milliseconds,
      consumer = { log ->
        if (log.blockNumber == 20UL && throwErrorLatch.get()) {
          failureCount.incrementAndGet()
          throw RuntimeException("Simulated failure")
        }
        consumedLogs.add(log)
      },
    )

    poller.start().get()

    // Then: all logs should eventually be consumed
    awaitUntilAsserted {
      assertThat(failureCount.get()).isGreaterThan(2)
      assertThat(consumedLogs).containsExactlyElementsOf(listOf(logs[0]))
    }

    throwErrorLatch.set(false)

    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactlyElementsOf(logs)
    }
  }

  @Test
  fun `should handle empty log results and continue polling when toBlock is a tag (FINALIZED, SAFE, LATEST)`() {
    // Given: no logs initially
    fakeEthApiClient.setLogs(emptyList())
    fakeEthApiClient.setFinalizedBlockTag(10UL)

    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      pollingInterval = 100.milliseconds,
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()

    // Wait a bit to ensure polling happens with no logs
    Thread.sleep(250)

    // Then: no logs consumed yet
    assertThat(consumedLogs).isEmpty()

    // When: finalized block advances and logs are added in the new range
    val newLog = templateLog.copy(blockNumber = 20UL, logIndex = 0UL)
    fakeEthApiClient.addLogs(setOf(newLog))
    fakeEthApiClient.setFinalizedBlockTag(30UL)

    // Then: new log should be consumed
    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactly(newLog)
    }
  }

  @Test
  fun `should handle case when fromBlock is greater than or equal to toBlock`() {
    // Given: finalized block is behind current search position
    fakeEthApiClient.setFinalizedBlockTag(10UL)

    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(20UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      pollingInterval = 100.milliseconds,
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()
    Thread.sleep(250)

    // Then: no logs should be consumed
    assertThat(consumedLogs).isEmpty()

    // When: finalized block moves forward
    val newLog = templateLog.copy(blockNumber = 25UL, logIndex = 0UL)
    fakeEthApiClient.addLogs(setOf(newLog))
    fakeEthApiClient.setFinalizedBlockTag(30UL)

    // Then: log should be consumed
    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactly(newLog)
    }
  }

  @Test
  fun `should respect blockChunkSize parameter`() {
    // Given: many logs across wide block range
    val logs = (0UL..100UL step 5).map { blockNum ->
      templateLog.copy(blockNumber = blockNum, logIndex = 0UL)
    }
    fakeEthApiClient.setLogs(logs)
    fakeEthApiClient.setFinalizedBlockTag(150UL)

    // When: poller uses small chunk size
    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      blockChunkSize = 10u,
      pollingInterval = 50.milliseconds,
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()

    // Then: all logs should be consumed
    await()
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(consumedLogs).containsExactlyElementsOf(logs)
      }
  }

  @Test
  fun `should handle logs with same blockNumber but different logIndex`() {
    // Given: multiple logs at same block with different indices
    val logs = listOf(
      templateLog.copy(blockNumber = 10UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 10UL, logIndex = 1UL),
      templateLog.copy(blockNumber = 10UL, logIndex = 2UL),
      templateLog.copy(blockNumber = 20UL, logIndex = 0UL),
    )
    fakeEthApiClient.setLogs(logs)
    fakeEthApiClient.setFinalizedBlockTag(50UL)

    // When: logs are consumed
    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()

    // Then: all logs should be consumed in order
    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactlyElementsOf(logs)
    }
  }

  @Test
  fun `should filter logs by address and topics`() {
    // Given: logs with different addresses and topics
    val matchingLogs = listOf(
      templateLog.copy(
        blockNumber = 10UL,
        logIndex = 0UL,
        topics = listOf(testTopic1.decodeHex()),
      ),
      templateLog.copy(
        blockNumber = 20UL,
        logIndex = 0UL,
        topics = listOf(testTopic1.decodeHex()),
      ),
    )
    val nonMatchingLogs = listOf(
      templateLog.copy(
        blockNumber = 15UL,
        logIndex = 0UL,
        topics = listOf(testTopic2.decodeHex()),
      ),
      templateLog.copy(
        blockNumber = 25UL,
        logIndex = 0UL,
        address = "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb".decodeHex(),
        topics = listOf(testTopic1.decodeHex()),
      ),
    )
    fakeEthApiClient.setLogs(matchingLogs + nonMatchingLogs)
    fakeEthApiClient.setFinalizedBlockTag(50UL)

    // When: poller filters by address and topic
    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      address = testAddress,
      topics = listOf(testTopic1),
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()

    // Then: only matching logs should be consumed
    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactlyElementsOf(matchingLogs)
    }
  }

  @Test
  fun `should handle start and stop lifecycle correctly`() {
    val logs1 = listOf(
      templateLog.copy(blockNumber = 10UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 20UL, logIndex = 1UL),
    )
    val logs2 = listOf(
      templateLog.copy(blockNumber = 300UL, logIndex = 2UL),
      templateLog.copy(blockNumber = 400UL, logIndex = 0UL),
    )
    fakeEthApiClient.setLogs(logs1)
    fakeEthApiClient.setFinalizedBlockTag(25UL)

    // Given: poller with consumer
    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      pollingInterval = 5.milliseconds,
      blockChunkSize = 5u,
      consumer = { log -> consumedLogs.add(log) },
    )

    poller.start().get()
    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactlyElementsOf(logs1)
    }
    poller.stop().get()
    fakeEthApiClient.addLogs(logs2.toSet())
    fakeEthApiClient.setFinalizedBlockTag(500UL)

    Thread.sleep(200)
    assertThat(consumedLogs).containsExactlyElementsOf(logs1)
  }

  @Test
  fun `should update lastSearchedBlock when searching single block with no logs`() {
    // Given: no logs at block 10, but a log exists at block 11
    val logAtBlock11 = templateLog.copy(blockNumber = 11UL, logIndex = 0UL)
    fakeEthApiClient.setLogs(listOf(logAtBlock11))
    fakeEthApiClient.setFinalizedBlockTag(10UL) // Only block 10 is finalized (no logs)

    val consumedLogs = mutableListOf<EthLog>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(10UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      pollingInterval = 10.milliseconds,
      consumer = { log -> consumedLogs.add(log) },
    )

    // When: poller starts and searches block 10 multiple times (single block range, no logs)
    poller.start().get()

    // Allow multiple poll cycles to occur on the single empty block
    // With the bug: each poll searches block 10 again because lastSearchedBlock isn't updated
    Thread.sleep(200)

    // Then: no logs consumed yet (block 11 is not finalized)
    assertThat(consumedLogs).isEmpty()

    // When: finalized block advances to 11, making the log available
    fakeEthApiClient.setFinalizedBlockTag(11UL)

    awaitUntilAsserted {
      assertThat(consumedLogs).containsExactly(logAtBlock11)
    }
  }

  @Test
  fun `should notify state listener when transitioning between states`() {
    // Given: logs exist up to block 20
    val logs = listOf(
      templateLog.copy(blockNumber = 10UL, logIndex = 0UL),
      templateLog.copy(blockNumber = 20UL, logIndex = 0UL),
    )
    fakeEthApiClient.setLogs(logs)
    fakeEthApiClient.setFinalizedBlockTag(30UL)

    // When: poller starts with state listener
    val stateTransitions = mutableListOf<Pair<EthLogsFilterState, EthLogsFilterState>>()
    poller = createPoller(
      fromBlock = BlockParameter.BlockNumber(0UL),
      toBlock = BlockParameter.Tag.FINALIZED,
      pollingInterval = 50.milliseconds,
      consumer = { },
    )
    poller.setStateListener { oldState, newState ->
      stateTransitions.add(oldState to newState)
    }

    poller.start().get()

    // Then: should transition from Idle to Searching first
    awaitUntilAsserted {
      assertThat(stateTransitions.first())
        .isEqualTo(EthLogsFilterState.Idle to EthLogsFilterState.Searching(0UL))
    }

    // Then: should eventually transition to CaughtUp
    awaitUntilAsserted {
      assertThat(stateTransitions).hasSizeGreaterThan(1)
      assertThat(stateTransitions[1])
        .isEqualTo(EthLogsFilterState.Searching(0UL) to EthLogsFilterState.CaughtUp(30UL))
    }

    fakeEthApiClient.setFinalizedBlockTag(50UL)
    awaitUntilAsserted {
      assertThat(stateTransitions).hasSizeGreaterThan(2)
      assertThat(stateTransitions[2])
        .isEqualTo(EthLogsFilterState.CaughtUp(30UL) to EthLogsFilterState.Searching(31UL))
    }
  }

  private fun createPoller(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String = testAddress,
    topics: List<String> = emptyList(),
    blockChunkSize: UInt = 1000u,
    pollingInterval: kotlin.time.Duration = 10.milliseconds,
    consumer: ((EthLog) -> Unit)? = null,
  ): EthLogsFilterPoller {
    val filterOptions = EthLogsFilterOptions(
      fromBlock = fromBlock,
      toBlock = toBlock,
      address = address,
      topics = topics,
    )

    return EthLogsFilterPoller(
      vertx = vertx,
      ethApiClient = fakeEthApiClient,
      filterOptions = filterOptions,
      l1FtxLogsPollingInterval = pollingInterval,
      blockChunkSize = blockChunkSize,
    ).apply {
      consumer?.let { setConsumer(it) }
    }
  }

  private fun awaitUntilAsserted(fn: () -> Unit) {
    await()
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted(fn)
  }
}
