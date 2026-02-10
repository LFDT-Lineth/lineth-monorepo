package net.consensys.zkevm.ethereum.coordination.conflation

import linea.domain.BlockHeaderSummary
import linea.kotlin.ByteArrayExt
import net.consensys.FakeFixedClock
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.fakeTracesCountersV2
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.blob.FakeBlobCompressor
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.mock
import org.mockito.kotlin.spy
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.days
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class GlobalBlockConflationCalculatorIntTest {
  // NOTE: this breaks the test isolation, but adds some confidence that the integration works
  private lateinit var calculatorByDealine: ConflationCalculatorByTimeDeadline
  private lateinit var calculatorByTraces: ConflationCalculator
  private lateinit var calculatorByHardFork: ConflationCalculator
  private lateinit var globalCalculator: GlobalBlockConflationCalculator
  private val lastBlockNumber: ULong = 0uL
  private lateinit var safeBlockProvider: SafeBlockProvider
  private lateinit var fakeClock: FakeFixedClock
  private val fakeClockTime = Instant.parse("2023-12-11T00:00:00.000Z")
  private lateinit var conflations: MutableList<ConflationCalculationResult>

  @BeforeEach
  fun beforeEach() {
    fakeClock = FakeFixedClock(fakeClockTime)
    conflations = mutableListOf()
    safeBlockProvider =
      mock<SafeBlockProvider> {
        on { getLatestSafeBlock() }.thenReturn(
          SafeFuture.failedFuture(RuntimeException("getLatestSafeBlock should not be called")),
        )
        on { getLatestSafeBlockHeader() }.thenReturn(
          SafeFuture.failedFuture(RuntimeException("getLatestSafeBlockHeader not mocked yet")),
        )
      }
    calculatorByDealine =
      spy(
        ConflationCalculatorByTimeDeadline(
          config =
          ConflationCalculatorByTimeDeadline.Config(
            conflationDeadline = 2.seconds,
            conflationDeadlineLastBlockConfirmationDelay = 10.milliseconds,
          ),
          clock = fakeClock,
          lastBlockNumber = lastBlockNumber,
          latestBlockProvider = safeBlockProvider,
        ),
      )
    val fakeBlobCompressor = FakeBlobCompressor(1_000)
    val calculatorByData =
      spy(
        ConflationCalculatorByDataCompressed(
          blobCompressor = fakeBlobCompressor,
        ),
      )
    whenever(calculatorByData.reset()).then {
      fakeBlobCompressor.reset()
    }

    calculatorByTraces =
      ConflationCalculatorByExecutionTraces(
        tracesCountersLimit = fakeTracesCountersV2(100u),
        emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
        metricsFacade = mock<MetricsFacade>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
      )
    calculatorByHardFork =
      TimestampHardForkConflationCalculator(
        hardForkTimestamps =
        listOf(
          fakeClockTime.plus(10.seconds),
          fakeClockTime.plus(20.seconds),
          fakeClockTime.plus(30.seconds),
        ),
        initialTimestamp = fakeClock.now(),
      )
    globalCalculator =
      GlobalBlockConflationCalculator(
        lastBlockNumber = lastBlockNumber,
        syncCalculators = listOf(calculatorByTraces, calculatorByData, calculatorByHardFork),
        deferredTriggerConflationCalculators = listOf(calculatorByDealine),
        emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
      )
    globalCalculator.onConflatedBatch { trigger ->
      conflations.add(trigger)
      SafeFuture.completedFuture(Unit)
    }
  }

  @Test
  fun `conflation by traces limit - 1st block overflows`() {
    // block with traces oversize
    val block1Counters =
      BlockCounters(
        blockNumber = 1uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(101u),
        blockRLPEncoded = ByteArray(10),
      )
    val block2Counters =
      BlockCounters(
        blockNumber = 2uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(10),
      )
    globalCalculator.newBlock(block1Counters)
    globalCalculator.newBlock(block2Counters)

    assertThat(conflations).hasSize(1)
    assertThat(conflations[0]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 1uL,
        endBlockNumber = 1uL,
        conflationTrigger = ConflationTrigger.TRACES_LIMIT,
        tracesCounters = block1Counters.tracesCounters,
      ),
    )
  }

  @Test
  fun `conflation by traces limit - single block`() {
    // block with traces oversize
    val block1Counters =
      BlockCounters(
        blockNumber = 1uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(60u),
        blockRLPEncoded = ByteArray(10),
      )
    val block2Counters =
      BlockCounters(
        blockNumber = 2uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(50u),
        blockRLPEncoded = ByteArray(20),
      )
    globalCalculator.newBlock(block1Counters)
    globalCalculator.newBlock(block2Counters)

    assertThat(conflations).hasSize(1)
    assertThat(conflations[0]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 1uL,
        endBlockNumber = 1uL,
        conflationTrigger = ConflationTrigger.TRACES_LIMIT,
        tracesCounters = block1Counters.tracesCounters,
      ),
    )
  }

  @Test
  fun `conflation by traces limit - multiple blocks`() {
    // block with traces oversize
    val block1Counters =
      BlockCounters(
        blockNumber = 1uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(50u),
        blockRLPEncoded = ByteArray(10),
      )
    val block2Counters =
      BlockCounters(
        blockNumber = 2uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(50u),
        blockRLPEncoded = ByteArray(20),
      )
    val block3Counters =
      BlockCounters(
        blockNumber = 3uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(20),
      )
    globalCalculator.newBlock(block1Counters)
    globalCalculator.newBlock(block2Counters)
    globalCalculator.newBlock(block3Counters)

    assertThat(conflations).hasSize(1)
    assertThat(conflations[0]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 1uL,
        endBlockNumber = 2uL,
        conflationTrigger = ConflationTrigger.TRACES_LIMIT,
        tracesCounters = block1Counters.tracesCounters.add(block2Counters.tracesCounters),
      ),
    )
  }

  @Test
  fun `conflation by data limit`() {
    // block with traces oversize
    val block1Counters =
      BlockCounters(
        blockNumber = 1uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(500),
      )
    val block2Counters =
      BlockCounters(
        blockNumber = 2uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(20u),
        blockRLPEncoded = ByteArray(480),
      )
    val block3Counters =
      BlockCounters(
        blockNumber = 3uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(20u),
        blockRLPEncoded = ByteArray(21),
      )
    globalCalculator.newBlock(block1Counters)
    globalCalculator.newBlock(block2Counters)
    globalCalculator.newBlock(block3Counters)

    assertThat(conflations).hasSize(1)
    assertThat(conflations[0]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 1uL,
        endBlockNumber = 2uL,
        conflationTrigger = ConflationTrigger.DATA_LIMIT,
        tracesCounters = block1Counters.tracesCounters.add(block2Counters.tracesCounters),
      ),
    )
  }

  @Test
  fun `conflation by hard fork`() {
    val block1Counters =
      BlockCounters(
        blockNumber = 1uL,
        blockTimestamp = fakeClock.now() + 9.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(10),
      )
    val block2Counters =
      BlockCounters(
        blockNumber = 2uL,
        blockTimestamp = fakeClock.now() + 10.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(10),
      )
    val block3Counters =
      BlockCounters(
        blockNumber = 3uL,
        blockTimestamp = fakeClock.now() + 19.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(10),
      )
    val block4Counters =
      BlockCounters(
        blockNumber = 4uL,
        blockTimestamp = fakeClock.now() + 30.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(10),
      )
    val block5Counters =
      BlockCounters(
        blockNumber = 5uL,
        blockTimestamp = fakeClock.now() + 31.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(10),
      )
    globalCalculator.newBlock(block1Counters) // first conflation as earlier than first hark-fork time
    globalCalculator.newBlock(block2Counters) // second conflation as earlier than second hark-fork time
    globalCalculator.newBlock(block3Counters) // second conflation as earlier than second hark-fork time
    globalCalculator.newBlock(block4Counters) // no conflation as equal to third hark-fork time
    globalCalculator.newBlock(block5Counters) // no conflation as later than third hark-fork time

    assertThat(conflations).hasSize(2)
    assertThat(conflations[0]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 1uL,
        endBlockNumber = 1uL,
        conflationTrigger = ConflationTrigger.HARD_FORK,
        tracesCounters = block1Counters.tracesCounters,
      ),
    )
    assertThat(conflations[1]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 2uL,
        endBlockNumber = 3uL,
        conflationTrigger = ConflationTrigger.HARD_FORK,
        tracesCounters = block2Counters.tracesCounters.add(block3Counters.tracesCounters),
      ),
    )
  }

  @Test
  fun `integrated flow with multiple scenarios`() {
    // block with traces oversize
    val block1Counters =
      BlockCounters(
        blockNumber = 1uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(101u),
        blockRLPEncoded = ByteArray(100),
      )
    // block with data in size limit
    val block2Counters =
      BlockCounters(
        blockNumber = 2uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(20u),
        blockRLPEncoded = ByteArray(1_000),
      )

    val block3Counters =
      BlockCounters(
        blockNumber = 3uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(30u),
        blockRLPEncoded = ByteArray(300),
      )
    val block4Counters =
      BlockCounters(
        blockNumber = 4uL,
        blockTimestamp = fakeClock.now(),
        tracesCounters = fakeTracesCountersV2(70u),
        blockRLPEncoded = ByteArray(400),
      )
    // will trigger traces overflow
    val block5Counters =
      BlockCounters(
        blockNumber = 5uL,
        blockTimestamp = fakeClock.now() + 9.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(100),
      )
    val block6Counters =
      BlockCounters(
        blockNumber = 6uL,
        blockTimestamp = fakeClock.now() + 19.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(100),
      )
    val block7Counters =
      BlockCounters(
        blockNumber = 7uL,
        blockTimestamp = fakeClock.now() + 30.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(100),
      )
    val block8Counters =
      BlockCounters(
        blockNumber = 8uL,
        blockTimestamp = fakeClock.now() + 31.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(100),
      )
    val block9Counters =
      BlockCounters(
        blockNumber = 9uL,
        blockTimestamp = fakeClock.now() + 32.seconds,
        tracesCounters = fakeTracesCountersV2(10u),
        blockRLPEncoded = ByteArray(100),
      )

    globalCalculator.newBlock(block1Counters)
    globalCalculator.newBlock(block2Counters)
    globalCalculator.newBlock(block3Counters)
    globalCalculator.newBlock(block4Counters)
    globalCalculator.newBlock(block5Counters)
    globalCalculator.newBlock(block6Counters)
    globalCalculator.newBlock(block7Counters)
    globalCalculator.newBlock(block8Counters)
    globalCalculator.newBlock(block9Counters)
    // will trigger deadline overflow
    fakeClock.advanceBy(2.days)

    whenever(safeBlockProvider.getLatestSafeBlockHeader()).thenReturn(
      SafeFuture.completedFuture(
        BlockHeaderSummary(
          number = 9uL,
          hash = ByteArrayExt.random32(),
          timestamp = block9Counters.blockTimestamp,
        ),
      ),
    )

    calculatorByDealine.checkConflationDeadline()

    assertThat(conflations).hasSize(6)

    assertThat(conflations[0]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 1uL,
        endBlockNumber = 1uL,
        conflationTrigger = ConflationTrigger.DATA_LIMIT,
        tracesCounters = block1Counters.tracesCounters,
      ),
    )
    assertThat(conflations[1]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 2uL,
        endBlockNumber = 2uL,
        conflationTrigger = ConflationTrigger.DATA_LIMIT,
        tracesCounters = block2Counters.tracesCounters,
      ),
    )
    assertThat(conflations[2]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 3uL,
        endBlockNumber = 4uL,
        conflationTrigger = ConflationTrigger.TRACES_LIMIT,
        tracesCounters = block3Counters.tracesCounters.add(block4Counters.tracesCounters),
      ),
    )
    assertThat(conflations[3]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 5uL,
        endBlockNumber = 5uL,
        conflationTrigger = ConflationTrigger.HARD_FORK,
        tracesCounters = block5Counters.tracesCounters,
      ),
    )
    assertThat(conflations[4]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 6uL,
        endBlockNumber = 6uL,
        conflationTrigger = ConflationTrigger.HARD_FORK,
        tracesCounters = block6Counters.tracesCounters,
      ),
    )
    assertThat(conflations[5]).isEqualTo(
      ConflationCalculationResult(
        startBlockNumber = 7uL,
        endBlockNumber = 9uL,
        conflationTrigger = ConflationTrigger.TIME_LIMIT,
        tracesCounters =
        block7Counters.tracesCounters.add(block8Counters.tracesCounters).add(block9Counters.tracesCounters),
      ),
    )
  }
}
