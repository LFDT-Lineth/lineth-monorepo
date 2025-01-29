package net.consensys.zkevm.ethereum.coordination.messageanchoring

import build.linea.contract.LineaRollupV6
import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import org.web3j.abi.EventEncoder
import org.web3j.abi.FunctionEncoder
import org.web3j.abi.TypeEncoder
import org.web3j.abi.datatypes.Address
import org.web3j.abi.datatypes.generated.Uint256
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.request.EthFilter
import org.web3j.protocol.core.methods.response.EthBlock
import org.web3j.protocol.core.methods.response.EthLog
import org.web3j.protocol.core.methods.response.EthLog.LogResult
import org.web3j.protocol.core.methods.response.Log
import java.math.BigInteger
import java.util.concurrent.TimeUnit
import kotlin.random.Random
import kotlin.random.nextUInt
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class L1EventQuerierImplTest {
  private val testContractAddress = "0x6d976c9b8ceee705d4fe8699b44e5eb58242f484"
  private val testToAddress: Address = Address("0x087b027b0573D4f01345eF8D081E0E7d3B378d14")
  private val testFromAddress = Address("0x95222290DD7278Aa3Ddd389Cc1E1d165CC4BAfe5")
  private val testFee = Uint256(12345)
  private val testValue = Uint256(123456789)
  private val testNonce = Uint256(1)
  private val callData: org.web3j.abi.datatypes.DynamicBytes =
    org.web3j.abi.datatypes.DynamicBytes("".encodeToByteArray())

  private val logIndexStart = 1
  private val blockInitialEventIsOn = 19
  private val maxMessagesToAnchor = 100u
  private val pollingInterval = 10.milliseconds
  private val earliestL1Block = BigInteger.valueOf(0)
  private val maxEventScrapingTime: Duration = 1.seconds
  private val blockRangeLoopLimit = 100u

  private lateinit var l1ClientMock: Web3j
  private lateinit var mockedEthBlock: EthBlock
  private lateinit var l1EventQuerier: L1EventQuerier

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    l1ClientMock = mock<Web3j>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    mockedEthBlock = mock<EthBlock>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    l1EventQuerier =
      L1EventQuerierImpl(
        vertx,
        L1EventQuerierImpl.Config(
          pollingInterval,
          maxEventScrapingTime,
          earliestL1Block,
          maxMessagesToAnchor,
          testContractAddress,
          "latest",
          blockRangeLoopLimit
        ),
        l1ClientMock
      )
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun nullHashAndNoMessages_returnsNoEvents(testContext: VertxTestContext) {
    whenever(mockedEthBlock.block.number).thenReturn(BigInteger.valueOf(20))

    whenever(l1ClientMock.ethGetBlockByNumber(any(), any()).send())
      .thenReturn(mockedEthBlock)

    val emptyEvents: List<LogResult<Log>> = listOf()
    val mockLogs = mock<EthLog>()
    whenever(mockLogs.logs).thenReturn(emptyEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs)

    l1EventQuerier.getSendMessageEventsForAnchoredMessage(null).thenApply {
      testContext
        .verify {
          assertThat(it).isNotNull
          assertThat(it).isEmpty()
        }
        .completeNow()
    }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(2, timeUnit = TimeUnit.SECONDS)
  fun nullHashAndMoreThan100Messages_returns100Events(testContext: VertxTestContext) {
    whenever(mockedEthBlock.block.number).thenReturn(BigInteger.valueOf(20))

    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val mockLogs = mock<EthLog>()
    val events = (blockInitialEventIsOn + 1..blockInitialEventIsOn + 106).map {
      createRandomSendEvent(
        it.toString(),
        Random.nextUInt().toString()
      )
    }
    whenever(mockLogs.logs).thenReturn(events)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs)

    l1EventQuerier.getSendMessageEventsForAnchoredMessage(null).thenApply {
      testContext
        .verify {
          assertThat(it).isNotNull
          assertThat(it.count()).isEqualTo(100)
        }
        .completeNow()
    }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun nullHashAndLessThan100Messages_returnsLessThan100Events(
    testContext: VertxTestContext
  ) {
    whenever(mockedEthBlock.block.number)
      .thenReturn(BigInteger.valueOf(20))
      .thenReturn(BigInteger.valueOf(100))
    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val mockLogs = mock<EthLog>()
    val events = (blockInitialEventIsOn + 1..blockInitialEventIsOn + 80).map {
      createRandomSendEvent(
        it.toString(),
        Random.nextUInt().toString()
      )
    }
    whenever(mockLogs.logs).thenReturn(events)

    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs).thenReturn(mock<EthLog>())

    l1EventQuerier.getSendMessageEventsForAnchoredMessage(null).thenApply {
      testContext
        .verify {
          assertThat(it).isNotNull
          assertThat(it.count()).isEqualTo(80)
        }
        .completeNow()
    }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun nullHashAndMoreThan100MessagesInMultipleQueries_returns100Events(
    testContext: VertxTestContext
  ) {
    whenever(mockedEthBlock.block.number)
      .thenReturn(BigInteger.valueOf(20))
      .thenReturn(BigInteger.valueOf(140))
    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val mockLogs = mock<EthLog>()
    val events = (blockInitialEventIsOn + 1..blockInitialEventIsOn + 80).map {
      createRandomSendEvent(
        it.toString(),
        Random.nextUInt().toString()
      )
    }
    whenever(mockLogs.logs).thenReturn(events)

    val mockLogsRound2 = mock<EthLog>()
    val eventsRound2 = (blockInitialEventIsOn + 100..blockInitialEventIsOn + 120).map {
      createRandomSendEvent(
        it.toString(),
        Random.nextUInt().toString()
      )
    }
    whenever(mockLogsRound2.logs).thenReturn(eventsRound2)

    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs).thenReturn(mockLogsRound2)

    l1EventQuerier.getSendMessageEventsForAnchoredMessage(null).thenApply {
      testContext
        .verify {
          assertThat(it).isNotNull
          assertThat(it.count()).isEqualTo(100)
        }
        .completeNow()
    }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun existingHashNotFoundAndNoMessages_returnsNoEvents(
    testContext: VertxTestContext
  ) {
    whenever(mockedEthBlock.block.number).thenReturn(BigInteger.valueOf(20))

    whenever(l1ClientMock.ethGetBlockByNumber(any(), any()).send())
      .thenReturn(mockedEthBlock)

    val emptyEvents: List<LogResult<Log>> = listOf()
    val mockLogs = mock<EthLog>()
    whenever(mockLogs.logs).thenReturn(emptyEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(
        MessageHashAnchoredEvent(messageHash = Bytes32.random())
      )
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it).isEmpty()
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun existingHashFoundAndNoMessages_returnsNoEvents(testContext: VertxTestContext) {
    val messageHash = Bytes32.random()
    whenever(mockedEthBlock.block.number)
      .thenReturn(BigInteger.valueOf(20))
      .thenReturn(BigInteger.valueOf(30))

    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val mockLogs = mock<EthLog>()
    val emptyEvents: List<LogResult<Log>> = listOf()
    whenever(mockLogs.logs).thenReturn(emptyEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs)

    val eventMockLogs = mock<EthLog>()
    val initialEvent = createRandomSendEvent(blockInitialEventIsOn.toString(), Random.nextUInt().toString())
    whenever(eventMockLogs.logs).thenReturn(listOf(initialEvent))
    whenever(l1ClientMock.ethGetLogs(buildMessageHashEventFilter(messageHash)).send()).thenReturn(eventMockLogs)
      .thenReturn(mockLogs)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(messageHash))
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it).isEmpty()
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun existingHashFoundMoreThan100Messages_returns100Events(
    testContext: VertxTestContext
  ) {
    val messageHash = Bytes32.random()
    whenever(mockedEthBlock.block.number)
      .thenReturn(BigInteger.valueOf(20))
      .thenReturn(BigInteger.valueOf(30))

    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val mockLogs = mock<EthLog>()
    val newEvents = (blockInitialEventIsOn + 1..blockInitialEventIsOn + 100)
      .map { createRandomSendEvent(it.toString(), Random.nextUInt().toString()) }
    whenever(mockLogs.logs).thenReturn(newEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs)

    val eventMockLogs = mock<EthLog>()
    val initialEvent = createRandomSendEvent(blockInitialEventIsOn.toString(), Random.nextUInt().toString())
    whenever(eventMockLogs.logs).thenReturn(listOf(initialEvent))
    whenever(l1ClientMock.ethGetLogs(buildMessageHashEventFilter(messageHash)).send()).thenReturn(eventMockLogs)
      .thenReturn(mockLogs)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(messageHash))
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.count()).isEqualTo(100)
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun existingHashFoundLessThan100Messages_returnsLessThan100Events(
    testContext: VertxTestContext
  ) {
    val messageHash = Bytes32.random()
    whenever(mockedEthBlock.block.number)
      .thenReturn(BigInteger.valueOf(20))
      .thenReturn(BigInteger.valueOf(30))

    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val mockLogs = mock<EthLog>()
    val newEvents = (blockInitialEventIsOn + 1..blockInitialEventIsOn + 80).map {
      createRandomSendEvent(
        it.toString(),
        Random.nextUInt().toString()
      )
    }
    whenever(mockLogs.logs).thenReturn(newEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs)

    val emptyLogs = mock<EthLog>()
    val emptyEvents: List<LogResult<Log>> = listOf()
    whenever(emptyLogs.logs).thenReturn(emptyEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(emptyLogs)

    val eventMockLogs = mock<EthLog>()
    val initialEvent = createRandomSendEvent(blockInitialEventIsOn.toString(), Random.nextUInt().toString())
    whenever(eventMockLogs.logs).thenReturn(listOf(initialEvent))
    whenever(l1ClientMock.ethGetLogs(buildMessageHashEventFilter(messageHash)).send())
      .thenReturn(eventMockLogs)
      .thenReturn(mockLogs).thenReturn(emptyLogs)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(messageHash))
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.count()).isEqualTo(80)
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun existingHashFound_DoesNotReturnDuplicateHashesWhenFinalBlockIsAlwaysTheSame(
    testContext: VertxTestContext
  ) {
    val startingIndexForEvents = 1
    val expectedEventCount = 20
    val finalBlockThatDoesNotChange = blockInitialEventIsOn + 1
    val messageHash = Bytes32.random()

    whenever(l1ClientMock.ethGetBlockByNumber(any(), any()).send().block.number)
      .thenReturn(BigInteger.valueOf(finalBlockThatDoesNotChange.toLong()))

    // all expected returned events, incrementing the log index
    val mockLogs = mock<EthLog>()
    val newEvents = (startingIndexForEvents..expectedEventCount).map {
      createRandomSendEvent(
        finalBlockThatDoesNotChange.toString(),
        it.toString()
      )
    }
    whenever(mockLogs.logs).thenReturn(newEvents)

    val foundEventLog = mock<EthLog>()
    val initialEvent = createRandomSendEvent(blockInitialEventIsOn.toString(), blockInitialEventIsOn.toString())
    whenever(foundEventLog.logs).thenReturn(listOf(initialEvent))

    whenever(l1ClientMock.ethGetLogs(any()).send())
      .thenReturn(foundEventLog).thenReturn(mockLogs)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(messageHash))
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.count()).isEqualTo(expectedEventCount)
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun existingHashFound_DoesNotReturnDuplicateHashesWhenFinalBlockIsTheSameRepeatedlyAndThenChanges(
    testContext: VertxTestContext
  ) {
    val expectedCountOnFirstFinalizedBlock = 20
    val expectedCountOnMovedOnFinalizedBlock = 20
    val finalBlockThatIsTheSameMultipleTimes = blockInitialEventIsOn + 1
    val movedOnFinalBlock = finalBlockThatIsTheSameMultipleTimes + 1
    val messageHash = Bytes32.random()
    val startingIndexForEvents = 1

    // return the same block multiple times, then move on
    whenever(l1ClientMock.ethGetBlockByNumber(any(), any()).send().block.number)
      .thenReturn(BigInteger.valueOf(finalBlockThatIsTheSameMultipleTimes.toLong()))
      .thenReturn(BigInteger.valueOf(finalBlockThatIsTheSameMultipleTimes.toLong()))
      .thenReturn(BigInteger.valueOf(finalBlockThatIsTheSameMultipleTimes.toLong()))
      .thenReturn(BigInteger.valueOf(movedOnFinalBlock.toLong()))

    // all expected returned events, incrementing the log index
    val initialFinalizedBlockLogs = mock<EthLog>()
    val newEvents = (startingIndexForEvents..expectedCountOnFirstFinalizedBlock).map {
      createRandomSendEvent(
        finalBlockThatIsTheSameMultipleTimes.toString(),
        it.toString()
      )
    }
    whenever(initialFinalizedBlockLogs.logs).thenReturn(newEvents)

    val movedOnFinalizedLogs = mock<EthLog>()
    val movedOnEvents = (startingIndexForEvents..expectedCountOnMovedOnFinalizedBlock).map {
      createRandomSendEvent(
        movedOnFinalBlock.toString(),
        it.toString()
      )
    }
    whenever(movedOnFinalizedLogs.logs).thenReturn(movedOnEvents)

    val foundEventLog = mock<EthLog>()
    val initialEvent = createRandomSendEvent(blockInitialEventIsOn.toString(), blockInitialEventIsOn.toString())
    whenever(foundEventLog.logs).thenReturn(listOf(initialEvent))

    // return the same data for the same returned block multiple times, then move on
    whenever(l1ClientMock.ethGetLogs(any()).send())
      .thenReturn(foundEventLog)
      .thenReturn(initialFinalizedBlockLogs)
      .thenReturn(initialFinalizedBlockLogs)
      .thenReturn(initialFinalizedBlockLogs)
      .thenReturn(movedOnFinalizedLogs)
      .thenReturn(movedOnFinalizedLogs)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(messageHash))
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.count()).isEqualTo(expectedCountOnFirstFinalizedBlock + expectedCountOnMovedOnFinalizedBlock)
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun existingHashFound_returnsEventsOnLaterBlocksWithLowerLogIndex(
    testContext: VertxTestContext
  ) {
    val messageHash = Bytes32.random()
    whenever(mockedEthBlock.block.number)
      .thenReturn(BigInteger.valueOf(20))
      .thenReturn(BigInteger.valueOf(100))

    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val mockLogs = mock<EthLog>()
    val newEvents = (blockInitialEventIsOn + 1..blockInitialEventIsOn + 80).map {
      createRandomSendEvent(
        it.toString(),
        it.toString() // enforcing a lower index
      )
    }
    whenever(mockLogs.logs).thenReturn(newEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs)

    val emptyLogs = mock<EthLog>()
    val emptyEvents: List<LogResult<Log>> = listOf()
    whenever(emptyLogs.logs).thenReturn(emptyEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(emptyLogs)

    val eventMockLogs = mock<EthLog>()
    // Zenhub 770 - Enforcing a higher log index for the initial block to validate later blocks return results
    val initialEvent = createRandomSendEvent(blockInitialEventIsOn.toString(), "100") // all previous indexes are 20-99
    whenever(eventMockLogs.logs).thenReturn(listOf(initialEvent))
    whenever(l1ClientMock.ethGetLogs(buildMessageHashEventFilter(messageHash)).send())
      .thenReturn(eventMockLogs)
      .thenReturn(mockLogs).thenReturn(emptyLogs)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(messageHash))
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.count()).isEqualTo(80)
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun existingHashFound_returnsEventsWithHigherLogIndexOnSameBlock(
    testContext: VertxTestContext
  ) {
    val messageHash = Bytes32.random()
    whenever(mockedEthBlock.block.number)
      .thenReturn(BigInteger.valueOf(20))
      .thenReturn(BigInteger.valueOf(100))

    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val sameBlockNumber = Random.nextUInt()
    val mockLogs = mock<EthLog>()
    // have all the events in the same block
    val newEvents = (logIndexStart + 1..logIndexStart + 100).map {
      createRandomSendEvent(
        sameBlockNumber.toString(),
        it.toString()
      )
    }
    whenever(mockLogs.logs).thenReturn(newEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogs)

    val emptyLogs = mock<EthLog>()
    val emptyEvents: List<LogResult<Log>> = listOf()
    whenever(emptyLogs.logs).thenReturn(emptyEvents)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(emptyLogs)

    val eventMockLogs = mock<EthLog>()
    // Forcing a lower index on the same block
    val initialEvent = createRandomSendEvent(sameBlockNumber.toString(), logIndexStart.toString())
    whenever(eventMockLogs.logs).thenReturn(listOf(initialEvent))
    whenever(l1ClientMock.ethGetLogs(buildMessageHashEventFilter(messageHash)).send())
      .thenReturn(eventMockLogs)
      .thenReturn(mockLogs).thenReturn(emptyLogs)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(messageHash))
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.count()).isEqualTo(100)
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun foundHashAndMoreThan100MessagesInMultipleQueries_returns100Events(
    testContext: VertxTestContext
  ) {
    val messageHash = Bytes32.random()
    whenever(mockedEthBlock.block.number)
      .thenReturn(BigInteger.valueOf(20))
      .thenReturn(BigInteger.valueOf(130))

    whenever(l1ClientMock.ethGetBlockByNumber({ "latest" }, false).send())
      .thenReturn(mockedEthBlock)

    val mockLogs = mock<EthLog>()
    val newEvents = (blockInitialEventIsOn + 1..blockInitialEventIsOn + 80).map {
      createRandomSendEvent(
        it.toString(),
        Random.nextUInt().toString()
      )
    }
    whenever(mockLogs.logs).thenReturn(newEvents)

    val mockLogsRound2 = mock<EthLog>()
    val newEventsRound2 = (blockInitialEventIsOn + 100..blockInitialEventIsOn + 120).map {
      createRandomSendEvent(
        it.toString(),
        Random.nextUInt().toString()
      )
    }
    whenever(mockLogsRound2.logs).thenReturn(newEventsRound2)
    whenever(l1ClientMock.ethGetLogs(any()).send()).thenReturn(mockLogsRound2)

    val eventMockLogs = mock<EthLog>()
    val events = createRandomSendEvent(blockInitialEventIsOn.toString(), Random.nextUInt().toString())
    whenever(eventMockLogs.logs).thenReturn(listOf(events))
    whenever(l1ClientMock.ethGetLogs(buildMessageHashEventFilter(messageHash)).send()).thenReturn(eventMockLogs)
      .thenReturn(mockLogs).thenReturn(mockLogsRound2)

    l1EventQuerier
      .getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(messageHash))
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.count()).isEqualTo(100)
          }
          .completeNow()
      }.whenException(testContext::failNow)
  }

  private fun createRandomSendEvent(blockNumber: String, logIndex: String): LogResult<Log> {
    val log = Log()
    val eventSignature: String = EventEncoder.encode(LineaRollupV6.MESSAGESENT_EVENT)
    val messageHashValue = Bytes32.random()
    val messageHash = org.web3j.abi.datatypes.generated.Bytes32(messageHashValue.toArray())

    log.topics =
      listOf(
        eventSignature,
        TypeEncoder.encode(testFromAddress),
        TypeEncoder.encode(testToAddress),
        TypeEncoder.encode(messageHash)
      )

    log.data =
      FunctionEncoder.encodeConstructor(
        listOf(
          testFee,
          testValue,
          testNonce,
          callData
        )
      )

    log.setBlockNumber(blockNumber)
    log.setLogIndex(logIndex)

    return LogResult<Log> { log }
  }

  private fun buildMessageHashEventFilter(messageHash: Bytes32): EthFilter {
    val messageHashFilter =
      EthFilter(
        DefaultBlockParameter.valueOf(earliestL1Block),
        DefaultBlockParameter.valueOf(BigInteger.valueOf(20)),
        testContractAddress
      )

    messageHashFilter.addOptionalTopics(messageHash.toString())

    return messageHashFilter
  }
}
