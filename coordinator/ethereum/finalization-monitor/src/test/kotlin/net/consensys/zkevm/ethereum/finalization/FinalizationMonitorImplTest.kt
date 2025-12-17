package net.consensys.zkevm.ethereum.finalization

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import linea.ethapi.EthApiBlockClient
import linea.kotlin.ByteArrayExt
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito.RETURNS_DEEP_STUBS
import org.mockito.Mockito.atLeastOnce
import org.mockito.Mockito.verify
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import org.web3j.protocol.core.methods.response.EthBlockNumber
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicInteger
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class FinalizationMonitorImplTest {
  private val expectedBlockNumber = 2UL
  private val pollingInterval = 20.milliseconds
  private val config = FinalizationMonitorImpl.Config(pollingInterval)
  private lateinit var mockL2Client: EthApiBlockClient
  private lateinit var contractMock: LineaRollupSmartContractClientReadOnly
  private val mockBlockNumberReturn = mock<EthBlockNumber>()

  @BeforeEach
  fun setup() {
    mockL2Client = mock<EthApiBlockClient>(defaultAnswer = RETURNS_DEEP_STUBS)
    contractMock = mock<LineaRollupSmartContractClientReadOnly>(defaultAnswer = RETURNS_DEEP_STUBS)

    whenever(mockBlockNumberReturn.blockNumber).thenReturn(BigInteger.TWO)
  }

  @Test
  fun start_startsPollingProcess(vertx: Vertx, testContext: VertxTestContext) {
    whenever(contractMock.finalizedL2BlockNumber(eq(BlockParameter.Tag.FINALIZED)))
      .thenReturn(SafeFuture.completedFuture(expectedBlockNumber))
    val expectedStateRootHash = ByteArrayExt.random32()
    whenever(contractMock.blockStateRootHash(any(), any()))
      .thenReturn(SafeFuture.completedFuture(expectedStateRootHash))

    val expectedBlockHash = ByteArrayExt.random32()
    val mockBlockByNumberReturn = mock<BlockWithTxHashes>()
    whenever(mockBlockByNumberReturn.hash).thenReturn(expectedBlockHash)
    whenever(mockL2Client.ethGetBlockByNumberTxHashes(any())).thenAnswer {
      SafeFuture.completedFuture(mockBlockByNumberReturn)
    }
    val finalizationMonitorImpl =
      FinalizationMonitorImpl(
        config = config,
        contract = contractMock,
        l2EthApiClient = mockL2Client,
        vertx = vertx,
      )
    finalizationMonitorImpl
      .start()
      .thenApply {
        await()
          .atMost(5.seconds.toJavaDuration())
          .untilAsserted {
            verify(
              mockL2Client,
              atLeastOnce(),
            ).ethGetBlockByNumberTxHashes(eq(BlockParameter.fromNumber(expectedBlockNumber)))
            verify(contractMock, atLeastOnce()).finalizedL2BlockNumber(BlockParameter.Tag.FINALIZED)
            verify(contractMock, atLeastOnce()).blockStateRootHash(BlockParameter.Tag.FINALIZED, expectedBlockNumber)
          }
      }
      .thenCompose { finalizationMonitorImpl.stop() }
      .thenApply { testContext.completeNow() }
      .whenException(testContext::failNow)
  }

  @Test
  fun finalizationUpdatesAreSentToTheRightHandlers(vertx: Vertx, testContext: VertxTestContext) {
    var blockNumber = 0
    whenever(contractMock.finalizedL2BlockNumber(any())).thenAnswer {
      blockNumber += 1
      SafeFuture.completedFuture(blockNumber.toULong())
    }
    whenever(contractMock.blockStateRootHash(any(), any())).thenAnswer {
      SafeFuture.completedFuture(intToBytes32(blockNumber).toArray())
    }

    val expectedBlockHash = intToBytes32(blockNumber).toArray()
    val mockBlockByNumberReturn = mock<BlockWithTxHashes>()
    whenever(mockBlockByNumberReturn.hash).thenReturn(expectedBlockHash)
    whenever(mockL2Client.ethGetBlockByNumberTxHashes(any())).thenAnswer {
      SafeFuture.completedFuture(mockBlockByNumberReturn)
    }

    val finalizationMonitorImpl =
      FinalizationMonitorImpl(
        config = config,
        contract = contractMock,
        l2EthApiClient = mockL2Client,
        vertx = vertx,
      )
    val updatesReceived1 = mutableListOf<FinalizationMonitor.FinalizationUpdate>()
    val updatesReceived2 = mutableListOf<FinalizationMonitor.FinalizationUpdate>()
    finalizationMonitorImpl.addFinalizationHandler("handler1") {
      SafeFuture.completedFuture(updatesReceived1.add(it))
    }
    val secondFinalisationHandlerSet = testContext.checkpoint()
    finalizationMonitorImpl
      .start()
      .thenApply {
        secondFinalisationHandlerSet.flag()
        await()
          .untilAsserted {
            finalizationMonitorImpl.addFinalizationHandler("handler2") {
              SafeFuture.completedFuture(updatesReceived2.add(it))
            }
          }
        await()
          .untilAsserted {
            finalizationMonitorImpl
              .stop()
              .thenApply {
                assertThat(updatesReceived1).isNotEmpty
                assertThat(updatesReceived2).isNotEmpty
                assertThat(updatesReceived1).isNotEqualTo(updatesReceived2)
                assertThat(updatesReceived2).contains(updatesReceived1.last())

                testContext.completeNow()
              }
              .whenException(testContext::failNow)
          }
      }.whenException(testContext::failNow)
  }

  @Test
  fun finalizationUpdatesDontCrashTheWholeMonitorInCaseOfErrors(vertx: Vertx, testContext: VertxTestContext) {
    var blockNumber = 0
    whenever(contractMock.finalizedL2BlockNumber(any())).thenAnswer {
      blockNumber += 1
      SafeFuture.completedFuture(blockNumber.toULong())
    }
    whenever(contractMock.blockStateRootHash(any(), any())).thenAnswer {
      SafeFuture.completedFuture(intToBytes32(blockNumber).toArray())
    }

    val expectedBlockHash = intToBytes32(blockNumber).toArray()
    val mockBlockByNumberReturn = mock<BlockWithTxHashes>()
    whenever(mockBlockByNumberReturn.hash).thenReturn(expectedBlockHash)
    whenever(mockL2Client.ethGetBlockByNumberTxHashes(any())).thenAnswer {
      SafeFuture.completedFuture(mockBlockByNumberReturn)
    }

    val finalizationMonitorImpl =
      FinalizationMonitorImpl(
        config = config,
        contract = contractMock,
        l2EthApiClient = mockL2Client,
        vertx = vertx,
      )
    val updatesReceived = CopyOnWriteArrayList<FinalizationMonitor.FinalizationUpdate>()
    val numberOfEventsBeforeError = AtomicInteger(0)
    finalizationMonitorImpl.addFinalizationHandler("handler1") {
      SafeFuture.completedFuture(updatesReceived.add(it))
    }

    val errorsThrown = AtomicInteger(0)
    finalizationMonitorImpl.start()
    await()
      .pollInterval(10.milliseconds.toJavaDuration())
      .atMost(10.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(updatesReceived).hasSizeGreaterThanOrEqualTo(1)
        finalizationMonitorImpl.addFinalizationHandler("handler2") {
          errorsThrown.incrementAndGet()
          numberOfEventsBeforeError.set(updatesReceived.size)
          throw Exception("Finalization callback failure for the testing!")
        }
      }

    await()
      .pollInterval(10.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(errorsThrown.get()).isGreaterThan(1)
      }

    await()
      .pollInterval(10.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(updatesReceived).hasSizeGreaterThanOrEqualTo(numberOfEventsBeforeError.get())
        testContext.completeNow()
      }

    finalizationMonitorImpl.stop().whenException(testContext::failNow)
  }

  @Test
  fun monitorDoesntCrashInCaseOfWeb3Exceptions(vertx: Vertx, testContext: VertxTestContext) {
    val updatesReceived = CopyOnWriteArrayList<FinalizationMonitor.FinalizationUpdate>()
    var blockNumber = 0
    var errorThrown = false
    var eventsReceivedBeforeError = 0
    whenever(contractMock.finalizedL2BlockNumber(any())).thenAnswer {
      blockNumber += 1
      if (blockNumber in 3..4) {
        errorThrown = true
        eventsReceivedBeforeError = updatesReceived.size
        throw Exception("Web3 client failure for the testing!")
      }
      SafeFuture.completedFuture(blockNumber.toULong())
    }
    whenever(contractMock.blockStateRootHash(any(), any())).thenAnswer {
      SafeFuture.completedFuture(intToBytes32(blockNumber).toArray())
    }

    val expectedBlockHash = intToBytes32(blockNumber).toArray()
    val mockBlockByNumberReturn = mock<BlockWithTxHashes>()
    whenever(mockBlockByNumberReturn.hash).thenReturn(expectedBlockHash)
    whenever(mockL2Client.ethGetBlockByNumberTxHashes(any())).thenAnswer {
      SafeFuture.completedFuture(mockBlockByNumberReturn)
    }

    val finalizationMonitorImpl =
      FinalizationMonitorImpl(
        config = config,
        contract = contractMock,
        l2EthApiClient = mockL2Client,
        vertx = vertx,
      )

    finalizationMonitorImpl.addFinalizationHandler("handler1") {
      SafeFuture.completedFuture(updatesReceived.add(it))
    }

    finalizationMonitorImpl.start()
    await()
      .untilAsserted {
        assertThat(updatesReceived).isNotEmpty
        assertThat(errorThrown).isTrue()
        assertThat(updatesReceived).hasSizeGreaterThan(eventsReceivedBeforeError)
      }

    finalizationMonitorImpl.stop()
      .thenApply { testContext.completeNow() }
  }

  @Test
  fun finalizationUpdatesHandledInOrderTheyWereSet(vertx: Vertx, testContext: VertxTestContext) {
    var blockNumber = 0
    whenever(contractMock.finalizedL2BlockNumber(any())).thenAnswer {
      blockNumber += 1
      SafeFuture.completedFuture(blockNumber.toULong())
    }
    whenever(contractMock.blockStateRootHash(any(), any())).thenAnswer {
      SafeFuture.completedFuture(intToBytes32(blockNumber).toArray())
    }

    val expectedBlockHash = intToBytes32(blockNumber).toArray()
    val mockBlockByNumberReturn = mock<BlockWithTxHashes>()
    whenever(mockBlockByNumberReturn.hash).thenReturn(expectedBlockHash)
    whenever(mockL2Client.ethGetBlockByNumberTxHashes(any())).thenAnswer {
      SafeFuture.completedFuture(mockBlockByNumberReturn)
    }

    val finalizationMonitorImpl =
      FinalizationMonitorImpl(
        config = config.copy(pollingInterval = pollingInterval * 2),
        contract = contractMock,
        l2EthApiClient = mockL2Client,
        vertx = vertx,
      )
    val updatesReceived = CopyOnWriteArrayList<Pair<FinalizationMonitor.FinalizationUpdate, String>>()

    fun simulateRandomWork(fromMillis: Long, toMillis: Long) {
      Thread.sleep(Random.nextLong(fromMillis, toMillis))
    }

    val handlerName1 = "handler1"
    finalizationMonitorImpl.addFinalizationHandler(handlerName1) { finalizationUpdate ->
      val result =
        SafeFuture.runAsync {
          simulateRandomWork(3, 7)
          updatesReceived.add(finalizationUpdate to handlerName1)
        }
      SafeFuture.of(result)
    }
    val handlerName2 = "handler2"
    finalizationMonitorImpl.addFinalizationHandler(handlerName2) { finalizationUpdate ->
      val result =
        SafeFuture.COMPLETE.thenApply {
          simulateRandomWork(2, 6)
          updatesReceived.add(finalizationUpdate to handlerName2)
        }
      SafeFuture.of(result)
    }
    val handlerName3 = "handler3"
    finalizationMonitorImpl.addFinalizationHandler(handlerName3) { finalizationUpdate ->
      val result =
        SafeFuture.COMPLETE.thenApply {
          simulateRandomWork(0, 4)
          updatesReceived.add(finalizationUpdate to handlerName3)
        }
      SafeFuture.of(result)
    }

    fun allUpdatesHandledInTheRightOrder() {
      assertThat(updatesReceived.size % 3).isZero()
        .overridingErrorMessage("Unexpected number of updates! $updatesReceived")
      assertThat(
        updatesReceived.windowed(3, 3).all { finalizationUpdates ->
          finalizationUpdates.map { it.second } == listOf(handlerName1, handlerName2, handlerName3)
        },
      )
        .overridingErrorMessage("Updates aren't in the right order! $updatesReceived")
        .isTrue()
    }

    finalizationMonitorImpl
      .start()
      .thenApply {
        await()
          .atMost(10.seconds.toJavaDuration())
          .pollInterval(200.milliseconds.toJavaDuration())
          .untilAsserted {
            allUpdatesHandledInTheRightOrder()
          }
        finalizationMonitorImpl.stop()
        testContext.completeNow()
      }
      .whenException(testContext::failNow)
  }

  private fun intToBytes32(i: Int): Bytes32 {
    return Bytes32.leftPad(Bytes.wrap(listOf(i.toByte()).toByteArray()))
  }
}
