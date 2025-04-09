package net.consensys.zkevm.ethereum.coordination.messageanchoring

import build.linea.contract.LineaRollupV6
import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.contract.l1.LineaContractVersion
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import net.consensys.linea.contract.LineaRollupAsyncFriendly
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.Web3jClientManager
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.web3j.abi.EventEncoder
import org.web3j.protocol.Web3j
import java.math.BigInteger
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class L1EventQuerierIntegrationTest {
  private lateinit var testLineaRollupContractAddress: String
  private val l2RecipientAddress = "0x03dfa322A95039BB679771346Ee2dBfEa0e2B773"
  private val blockRangeLoopLimit = 5u
  private val maxMessagesToCollect = 100u
  private lateinit var web3Client: Web3j
  private lateinit var contract: LineaRollupAsyncFriendly
  private var l1ContractDeploymentBlockNumber: ULong = 0u

  @BeforeEach
  fun beforeEach() {
    val deploymentResult = ContractsManager.get()
      .deployLineaRollup(contractVersion = LineaContractVersion.V6)
      .get()
    testLineaRollupContractAddress = deploymentResult.contractAddress
    web3Client = Web3jClientManager.l1Client
    @Suppress("DEPRECATION")
    contract = deploymentResult.rollupOperatorClientLegacy
    l1ContractDeploymentBlockNumber = deploymentResult.contractDeploymentBlockNumber.toULong()
  }

  private fun createL1EventQuerier(
    vertx: Vertx,
    blockRangeLoopLimit: UInt
  ): L1EventQuerierImpl {
    return L1EventQuerierImpl(
      vertx,
      L1EventQuerierImpl.Config(
        pollingInterval = 200.milliseconds,
        maxEventScrapingTime = 2.seconds,
        earliestL1Block = l1ContractDeploymentBlockNumber.toBigInteger(),
        maxMessagesToCollect = maxMessagesToCollect,
        l1MessageServiceAddress = testLineaRollupContractAddress,
        finalized = "latest",
        blockRangeLoopLimit = blockRangeLoopLimit
      ),
      web3Client
    )
  }

  @Test
  @Timeout(45, timeUnit = TimeUnit.SECONDS)
  fun `l1Event querier returns events from the given hash`(vertx: Vertx, testContext: VertxTestContext) {
    val baseMessageToSend =
      L1MessageToSend(l2RecipientAddress, BigInteger.ZERO, ByteArray(0), BigInteger.valueOf(100001))
    val messagesToSend = listOf(
      baseMessageToSend,
      baseMessageToSend.copy(value = BigInteger.valueOf(100001)),
      baseMessageToSend.copy(value = BigInteger.valueOf(100002)),
      baseMessageToSend.copy(value = BigInteger.valueOf(100003)),
      baseMessageToSend.copy(value = BigInteger.valueOf(100005))
    )

    val earlierEmittedParallelEvents = messagesToSend.map {
      contract.sendMessage(it.recipient, it.fee, it.calldata, it.value).sendAsync()
        .thenApply { receipt ->
          Pair(
            LineaRollupV6.getMessageSentEventFromLog(
              receipt.logs.first { log ->
                log.topics.contains(EventEncoder.encode(LineaRollupV6.MESSAGESENT_EVENT))
              }
            ),
            receipt
          )
        }
    }

    val earlierEvents = earlierEmittedParallelEvents.map { it.get() }

    val hashIndexToQueryFrom = 2
    val hashInTheMiddle = earlierEvents[hashIndexToQueryFrom].let {
      Bytes32.wrap(it.first._messageHash)
    }

    val laterEmittedEvents = messagesToSend.map {
      contract.sendMessage(it.recipient, it.fee, it.calldata, it.value).sendAsync()
        .thenApply { receipt ->
          Pair(
            LineaRollupV6.getMessageSentEventFromLog(
              receipt.logs.first { log ->
                log.topics.contains(EventEncoder.encode(LineaRollupV6.MESSAGESENT_EVENT))
              }
            ),
            receipt
          )
        }
    }

    val laterEvents = laterEmittedEvents.map { it.get() }
    val multiPostBlockLoopLimit = 20u
    val l1QuerierImpl = createL1EventQuerier(vertx, multiPostBlockLoopLimit)

    // we should have events 4 and 5 (count = 5, less hashIndexToQueryFrom, less 1 (0 based))
    val allExpectedHashesInOrder: MutableList<SendMessageEvent> =
      earlierEvents.drop(hashIndexToQueryFrom + 1).take(maxMessagesToCollect.toInt()).map {
        SendMessageEvent(
          Bytes32.wrap(it.first._messageHash),
          it.first._nonce.toULong(),
          it.first.log.blockNumber.toULong()

        )
      }.toMutableList()

    l1QuerierImpl.getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(hashInTheMiddle)).thenApply {
      // we should have events 1,2,3,4 and 5
      val expectedHashes = laterEvents.map {
        SendMessageEvent(
          Bytes32.wrap(it.first._messageHash),
          it.first._nonce.toULong(),
          it.first.log.blockNumber.toULong()
        )
      }.toMutableList()

      // we should have all 7
      allExpectedHashesInOrder.addAll(expectedHashes)

      testContext.verify {
        assertThat(it).isEqualTo(allExpectedHashesInOrder)
      }.completeNow()
    }.whenException(testContext::failNow)
  }

  @Disabled
  @Timeout(1, timeUnit = TimeUnit.MINUTES)
  fun `l1Event querier returns events with more than 10000 messages`(vertx: Vertx, testContext: VertxTestContext) {
    val baseMessageToSend =
      L1MessageToSend(l2RecipientAddress, BigInteger.ZERO, ByteArray(0), BigInteger.valueOf(100001))

    val messagesToSend = (1..10025).map { baseMessageToSend.copy(value = BigInteger.valueOf(100001)) }

    val earlierEmittedParallelEvents = messagesToSend.map {
      contract.sendMessage(it.recipient, it.fee, it.calldata, it.value).sendAsync()
        .thenApply { receipt ->
          Pair(
            LineaRollupV6.staticExtractEventParameters(
              LineaRollupV6.MESSAGESENT_EVENT,
              receipt.logs.first { log ->
                log.topics.contains(EventEncoder.encode(LineaRollupV6.MESSAGESENT_EVENT))
              }
            ),
            receipt
          )
        }
    }

    val events = earlierEmittedParallelEvents.map { it.get() }

    val hashIndexToQueryFrom = 2
    val hashToTake = events[hashIndexToQueryFrom].let {
      Bytes32.wrap(it.first.indexedValues[2].value as ByteArray)
    }

    val l1QuerierImpl = createL1EventQuerier(vertx, blockRangeLoopLimit)

    val allExpectedHashesInOrder =
      events.drop(hashIndexToQueryFrom + 1).take(maxMessagesToCollect.toInt()).map {
        Bytes32.wrap(it.first.indexedValues[2].value as ByteArray)
      }

    l1QuerierImpl.getSendMessageEventsForAnchoredMessage(MessageHashAnchoredEvent(hashToTake)).thenApply {
      val foundHashes = it.map { evt ->
        evt.messageHash
      }

      // we should have all the hashes meeting the maxMessagesToCollect
      testContext.verify {
        assertThat(it.count()).isEqualTo(maxMessagesToCollect)
        assertThat(foundHashes).isEqualTo(allExpectedHashesInOrder)
      }.completeNow()
    }.whenException(testContext::failNow)
  }
}
