package net.consensys.zkevm.ethereum.coordination.conflation

import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.domain.BlockHeaderSummary
import linea.kotlin.ByteArrayExt
import net.consensys.linea.traces.fakeTracesCountersV2
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.ArgumentMatchers.any
import org.mockito.ArgumentMatchers.contains
import org.mockito.Mockito
import org.mockito.kotlin.atLeast
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.spy
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

class ConflationCalculatorByTimeDeadlineTest {
  private val config = ConflationCalculatorByTimeDeadline.Config(
    conflationDeadline = 20.seconds,
    conflationDeadlineLastBlockConfirmationDelay = 4.seconds,
  )
  private val blockTimestamp = Instant.parse("2021-01-01T00:00:00Z")
  private val clock: Clock = mock { on { now() } doReturn blockTimestamp }
  private val latestBlockProvider: SafeBlockProvider = mock<SafeBlockProvider>() {
    on { getLatestSafeBlockHeader() } doReturn SafeFuture.completedFuture(
      BlockHeaderSummary(
        number = 1u,
        timestamp = blockTimestamp,
        hash = ByteArrayExt.random32(),
      ),
    )
  }
  private lateinit var conflationTiggers: MutableList<Instant>
  private lateinit var conflationCalculatorByTimeDeadline: ConflationCalculatorByTimeDeadline
  private lateinit var log: Logger

  @BeforeEach
  fun beforeEach() {
    conflationTiggers = mutableListOf<Instant>()
    log = mock()
    conflationCalculatorByTimeDeadline = spy(
      ConflationCalculatorByTimeDeadline(
        config = config,
        lastBlockNumber = 0u,
        clock = clock,
        latestBlockProvider = latestBlockProvider,
        log = log,
      ),
    ).also {
      it.setConflationTriggerConsumer {
        conflationTiggers.add(clock.now())
      }
    }
  }

  private fun checkDeadlineWasCalled(): Boolean {
    return Mockito.mockingDetails(conflationCalculatorByTimeDeadline).invocations
      .any { it.method.name == ConflationCalculatorByTimeDeadline::checkConflationDeadline.name }
  }

  @Test
  fun `should not trigger conflation when not blocks are inside`() {
    conflationCalculatorByTimeDeadline.checkConflationDeadline()
    assertThat(conflationTiggers).isEmpty()
  }

  @Test
  fun `should not trigger conflation before deadline`() {
    conflationCalculatorByTimeDeadline.appendBlock(blockCounters(1u, blockTimestamp))
    whenever(clock.now()).thenReturn(blockTimestamp.plus(config.conflationDeadline).minus(1.seconds))
    conflationCalculatorByTimeDeadline.checkConflationDeadline()
    assertThat(conflationTiggers).isEmpty()
  }

  @Test
  fun `should not trigger conflation after deadline when there are more blocks available to conflate`() {
    val block2Timestamp = blockTimestamp.plus(2.seconds)
    conflationCalculatorByTimeDeadline.appendBlock(blockCounters(1u, blockTimestamp))
    conflationCalculatorByTimeDeadline.appendBlock(blockCounters(2u, block2Timestamp))

    whenever(latestBlockProvider.getLatestSafeBlockHeader()).thenReturn(
      SafeFuture.completedFuture(
        BlockHeaderSummary(
          number = 3u,
          timestamp = block2Timestamp.plus(config.conflationDeadline).plus(5.seconds),
          hash = ByteArrayExt.random32(),
        ),
      ),
    )

    conflationCalculatorByTimeDeadline.checkConflationDeadline()
    assertThat(conflationTiggers).isEmpty()
  }

  @Test
  fun `should trigger conflation after deadline and there are no more blocks to conflate`() {
    val block2Timestamp = blockTimestamp.plus(2.seconds)
    conflationCalculatorByTimeDeadline.appendBlock(blockCounters(1u, blockTimestamp))
    conflationCalculatorByTimeDeadline.appendBlock(blockCounters(2u, block2Timestamp))

    whenever(latestBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = 2u,
            timestamp = block2Timestamp,
            hash = ByteArrayExt.random32(),
          ),
        ),
      )

    val time1 = blockTimestamp.plus(config.conflationDeadline).plus(10.seconds)
    whenever(clock.now()).thenReturn(time1)
    conflationCalculatorByTimeDeadline.checkConflationDeadline()
    assertThat(conflationTiggers).isEqualTo(listOf(time1))

    // it should reset internally, not more blocks should not trigger again.
    val time2 = time1.plus(config.conflationDeadline).plus(10.seconds)
    whenever(clock.now()).thenReturn(time2)
    conflationCalculatorByTimeDeadline.checkConflationDeadline()
    assertThat(conflationTiggers).isEqualTo(listOf(time1))
  }

  @Test
  fun `should set internal startTime when resetConflation is called`() {
    val block2Timestamp = blockTimestamp.plus(2.seconds)
    conflationCalculatorByTimeDeadline.appendBlock(blockCounters(1u, blockTimestamp))
    conflationCalculatorByTimeDeadline.appendBlock(blockCounters(2u, block2Timestamp))

    conflationCalculatorByTimeDeadline.reset()

    whenever(latestBlockProvider.getLatestSafeBlockHeader())
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = 2u,
            timestamp = block2Timestamp,
            hash = ByteArrayExt.random32(),
          ),
        ),
      )

    val time1 = blockTimestamp.plus(config.conflationDeadline).plus(10.seconds)
    whenever(clock.now()).thenReturn(time1)
    conflationCalculatorByTimeDeadline.checkConflationDeadline()
    assertThat(conflationTiggers).isEmpty()
  }

  @Test
  fun `should be resilient to failure fetching last block`() {
    conflationCalculatorByTimeDeadline.appendBlock(blockCounters(1u, blockTimestamp))

    whenever(latestBlockProvider.getLatestSafeBlockHeader())
      // it should be resilient to failures of fetching latest block
      .thenReturn(SafeFuture.failedFuture(RuntimeException("Failed to fetch latest block")))
      .thenReturn(
        SafeFuture.completedFuture(
          BlockHeaderSummary(
            number = 1u,
            timestamp = blockTimestamp,
            hash = ByteArrayExt.random32(),
          ),
        ),
      )

    whenever(clock.now()).thenReturn(blockTimestamp.plus(config.conflationDeadline).plus(10.seconds))
    conflationCalculatorByTimeDeadline.checkConflationDeadline() // 1st time will fail because request failed
    conflationCalculatorByTimeDeadline.checkConflationDeadline()
    assertThat(conflationTiggers).isEqualTo(listOf(clock.now()))
    verify(log).warn(
      eq("SafeBlock request failed. Will Retry conflation deadline on next tick errorMessage={}"),
      contains("Failed to fetch latest block"),
      any(),
    )
  }

  private fun blockCounters(blockNumber: ULong, timestamp: Instant): BlockCounters {
    return BlockCounters(
      blockNumber = blockNumber,
      blockTimestamp = timestamp,
      tracesCounters = fakeTracesCountersV2(1u),
      blockRLPEncoded = ByteArray(0),
    )
  }
}

class ConflationCalculatorByTimeDeadlineRunnerTest {
  val mockCalculator: ConflationCalculatorByTimeDeadline = mock()

  @Test
  fun `should call calculator every interval`() {
    val runner = DeadlineConflationCalculatorRunner(
      10.milliseconds,
      mockCalculator,
    )
    // it should be idempotent
    runner.start()
    runner.start()

    await()
      .atMost(1.seconds.toJavaDuration())
      .untilAsserted {
        verify(mockCalculator, atLeast(1)).checkConflationDeadline()
      }

    runner.stop()
    runner.stop()
  }
}
