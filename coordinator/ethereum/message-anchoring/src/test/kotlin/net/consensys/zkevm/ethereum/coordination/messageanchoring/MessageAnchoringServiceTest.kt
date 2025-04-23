package net.consensys.zkevm.ethereum.coordination.messageanchoring

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.L2MessageService
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import org.apache.tuweni.bytes.Bytes32
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito.RETURNS_DEEP_STUBS
import org.mockito.Mockito.atLeastOnce
import org.mockito.Mockito.verify
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.whenever
import org.web3j.protocol.core.methods.response.TransactionReceipt
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class MessageAnchoringServiceTest {
  private lateinit var mockTransactionManager: AsyncFriendlyTransactionManager
  private lateinit var mockL1Querier: L1EventQuerier
  private lateinit var mockL2MessageAnchorer: L2MessageAnchorer
  private lateinit var mockL2Querier: L2Querier
  private lateinit var l2MessageServiceContractClient: L2MessageService
  private lateinit var rollupSmartContractClient: LineaRollupSmartContractClient

  @BeforeEach
  fun beforeEach() {
    mockTransactionManager = mock<AsyncFriendlyTransactionManager>(defaultAnswer = RETURNS_DEEP_STUBS)
    mockL1Querier = mock<L1EventQuerier>(defaultAnswer = RETURNS_DEEP_STUBS)
    mockL2MessageAnchorer = mock<L2MessageAnchorer>(defaultAnswer = RETURNS_DEEP_STUBS)
    mockL2Querier = mock<L2Querier>(defaultAnswer = RETURNS_DEEP_STUBS)
    l2MessageServiceContractClient = mock<L2MessageService>(defaultAnswer = RETURNS_DEEP_STUBS)
    rollupSmartContractClient = mock<LineaRollupSmartContractClient>()
  }

  private fun createMessageAnchoringService(
    vertx: Vertx,
    maxMessagesToAnchor: UInt
  ): MessageAnchoringService {
    return MessageAnchoringService(
      MessageAnchoringService.Config(
        pollingInterval = 10.milliseconds,
        maxMessagesToAnchor = maxMessagesToAnchor
      ),
      vertx,
      mockL1Querier,
      mockL2MessageAnchorer,
      mockL2Querier,
      rollupSmartContractClient,
      l2MessageServiceContractClient,
      mockTransactionManager
    )
  }

  @Test
  @Timeout(4, timeUnit = TimeUnit.SECONDS)
  fun start_startsPollingProcessForMessagesUsingRollingHash(vertx: Vertx, testContext: VertxTestContext) {
    val maxMessagesToAnchor = 100u

    whenever(l2MessageServiceContractClient.INBOX_STATUS_UNKNOWN().send()).thenReturn(BigInteger.valueOf(0))

    val foundAnchoredHashEvent = MessageHashAnchoredEvent(Bytes32.random())
    val events = listOf(SendMessageEvent(Bytes32.random(), 10UL, 10UL))
    val mockTransactionReceipt = mock<TransactionReceipt>()

    whenever(mockL2Querier.findLastFinalizedAnchoredEvent()).thenReturn(
      SafeFuture.completedFuture(foundAnchoredHashEvent)
    )
    whenever(mockL1Querier.getSendMessageEventsForAnchoredMessage(foundAnchoredHashEvent)).thenReturn(
      SafeFuture.completedFuture(events)
    )
    whenever(rollupSmartContractClient.getMessageRollingHash(any(), any())).thenReturn(
      SafeFuture.completedFuture(Bytes32.ZERO.toArray())
    )
    whenever(mockL2Querier.getMessageHashStatus(events.first().messageHash)).thenReturn(
      SafeFuture.completedFuture(BigInteger.valueOf(0))
    )
    whenever(
      mockL2MessageAnchorer.anchorMessages(any(), any())
    ).thenReturn(
      SafeFuture.completedFuture(mockTransactionReceipt)
    )
    whenever(mockTransactionReceipt.transactionHash).thenReturn(
      Bytes32.random().toHexString()
    )

    whenever(mockTransactionManager.resetNonce()).thenAnswer { SafeFuture.completedFuture(Unit) }

    val monitor = createMessageAnchoringService(vertx, maxMessagesToAnchor)

    monitor.start().thenApply {
      vertx.setTimer(
        100
      ) {
        monitor.stop().thenApply {
          testContext.verify {
            verify(mockL2Querier, atLeastOnce()).findLastFinalizedAnchoredEvent()
            verify(mockL1Querier, atLeastOnce()).getSendMessageEventsForAnchoredMessage(foundAnchoredHashEvent)
            verify(mockL2Querier, atLeastOnce()).getMessageHashStatus(events.first().messageHash)
            verify(mockTransactionReceipt, atLeastOnce()).transactionHash
            verify(rollupSmartContractClient, atLeastOnce()).getMessageRollingHash(messageNumber = 10L)
            verify(mockL2MessageAnchorer, atLeastOnce()).anchorMessages(
              events,
              Bytes32.ZERO.toArray()
            )
            verify(mockTransactionManager, atLeastOnce()).resetNonce()
          }
            .completeNow()
        }
      }
    }
  }

  @Test
  @Timeout(4, timeUnit = TimeUnit.SECONDS)
  fun start_startsPollingProcessForMessagesUsingRollingHashAndLimitsReturnedEvents(
    vertx: Vertx,
    testContext: VertxTestContext
  ) {
    val maxMessagesToAnchor = 2u

    whenever(l2MessageServiceContractClient.INBOX_STATUS_UNKNOWN().send()).thenReturn(BigInteger.valueOf(0))

    val foundAnchoredHashEvent = MessageHashAnchoredEvent(Bytes32.random())
    val events = listOf(
      SendMessageEvent(Bytes32.random(), 1UL, 1UL),
      SendMessageEvent(Bytes32.random(), 2UL, 2UL),
      SendMessageEvent(Bytes32.random(), 3UL, 3UL),
      SendMessageEvent(Bytes32.random(), 4UL, 4UL)
    )

    val mockTransactionReceipt = mock<TransactionReceipt>()

    whenever(mockL2Querier.findLastFinalizedAnchoredEvent()).thenReturn(
      SafeFuture.completedFuture(foundAnchoredHashEvent)
    )
    whenever(mockL1Querier.getSendMessageEventsForAnchoredMessage(foundAnchoredHashEvent)).thenReturn(
      SafeFuture.completedFuture(events)
    )
    whenever(rollupSmartContractClient.getMessageRollingHash(any(), any())).thenReturn(
      SafeFuture.completedFuture(Bytes32.ZERO.toArray())
    )
    whenever(mockL2Querier.getMessageHashStatus(any())).thenReturn(
      SafeFuture.completedFuture(BigInteger.valueOf(0))
    )
    whenever(
      mockL2MessageAnchorer.anchorMessages(any(), any())
    ).thenReturn(
      SafeFuture.completedFuture(mockTransactionReceipt)
    )
    whenever(mockTransactionReceipt.transactionHash).thenReturn(
      Bytes32.random().toHexString()
    )

    whenever(mockTransactionManager.resetNonce()).thenAnswer { SafeFuture.completedFuture(Unit) }

    val monitor = createMessageAnchoringService(vertx, maxMessagesToAnchor)

    monitor.start().thenApply {
      vertx.setTimer(
        100
      ) {
        monitor.stop().thenApply {
          testContext.verify {
            verify(mockL2MessageAnchorer, atLeastOnce()).anchorMessages(
              events.take(2),
              Bytes32.ZERO.toArray()
            )
            verify(mockL2MessageAnchorer, never()).anchorMessages(
              events.take(4),
              Bytes32.ZERO.toArray()
            )
            verify(mockL2Querier, atLeastOnce()).getMessageHashStatus(events.first().messageHash)
            verify(mockL2Querier, atLeastOnce()).getMessageHashStatus(events[1].messageHash)
            verify(mockL2Querier, atLeastOnce()).getMessageHashStatus(events[2].messageHash)
            verify(mockL2Querier, atLeastOnce()).getMessageHashStatus(events[3].messageHash)
            verify(mockL2Querier, atLeastOnce()).findLastFinalizedAnchoredEvent()
            verify(rollupSmartContractClient, atLeastOnce()).getMessageRollingHash(messageNumber = 2L)
            verify(mockL1Querier, atLeastOnce()).getSendMessageEventsForAnchoredMessage(foundAnchoredHashEvent)
            verify(mockTransactionReceipt, atLeastOnce()).transactionHash
            verify(mockTransactionManager, atLeastOnce()).resetNonce()
          }
            .completeNow()
        }
      }
    }
  }
}
