package net.consensys.zkevm.ethereum.coordination.conflation

import kotlinx.datetime.Instant
import linea.domain.createBlock
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.fakeTracesCountersV2
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.awaitility.Awaitility
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito.RETURNS_DEEP_STUBS
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.time.Duration
import java.util.concurrent.Executors
import kotlin.time.Duration.Companion.seconds

class ConflationServiceImplTest {
  private val conflationBlockLimit = 2u
  private lateinit var conflationCalculator: TracesConflationCalculator
  private lateinit var conflationService: ConflationServiceImpl

  @BeforeEach
  fun beforeEach() {
    conflationCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = 0u,
      syncCalculators = listOf(
        ConflationCalculatorByBlockLimit(conflationBlockLimit),
      ),
      deferredTriggerConflationCalculators = emptyList(),
      emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
    )
    conflationService = ConflationServiceImpl(conflationCalculator, mock(defaultAnswer = RETURNS_DEEP_STUBS))
  }

  @Test
  fun `emits event with blocks when calculator emits conflation`() {
    val payload1 = createBlock(number = 1UL, gasLimit = 20_000_000UL)
    val payload2 = createBlock(number = 2UL, gasLimit = 20_000_000UL)
    val payload3 = createBlock(number = 3UL, gasLimit = 20_000_000UL)
    val payload1Time = Instant.parse("2021-01-01T00:00:00Z")
    val payloadCounters1 = BlockCounters(
      blockNumber = 1UL,
      payload1Time.plus(0.seconds),
      tracesCounters = fakeTracesCountersV2(40u),
      blockRLPEncoded = ByteArray(0),
    )
    val payloadCounters2 = BlockCounters(
      blockNumber = 2UL,
      payload1Time.plus(2.seconds),
      tracesCounters = fakeTracesCountersV2(40u),
      blockRLPEncoded = ByteArray(0),
    )
    val payloadCounters3 = BlockCounters(
      blockNumber = 3UL,
      payload1Time.plus(4.seconds),
      tracesCounters = fakeTracesCountersV2(100u),
      blockRLPEncoded = ByteArray(0),
    )

    val conflationEvents = mutableListOf<BlocksConflation>()
    conflationService.onConflatedBatch { conflationEvent: BlocksConflation ->
      conflationEvents.add(conflationEvent)
      SafeFuture.completedFuture(Unit)
    }

    // 1st conflation
    conflationService.newBlock(payload1, payloadCounters1)
    conflationService.newBlock(payload2, payloadCounters2)
    conflationService.newBlock(payload3, payloadCounters3)

    assertThat(conflationEvents).isEqualTo(
      listOf(
        BlocksConflation(
          listOf(payload1, payload2),
          ConflationCalculationResult(
            startBlockNumber = 1u,
            endBlockNumber = 2u,
            conflationTrigger = ConflationTrigger.BLOCKS_LIMIT,
            // these are not counted in conflation, so will be 0
            tracesCounters = fakeTracesCountersV2(0u),
          ),
        ),
      ),
    )
  }

  @Test
  fun `sends blocks in correct order to calculator`() {
    val numberOfThreads = 10
    val numberOfBlocks = 2000
    val moduleTracesCounter = 10u
    assertThat(numberOfBlocks % numberOfThreads).isEqualTo(0)
    val expectedConflations = numberOfBlocks / conflationBlockLimit.toInt() - 1
    val blocks = (1UL..numberOfBlocks.toULong()).map { createBlock(number = it, gasLimit = 20_000_000UL) }
    val fixedTracesCounters = fakeTracesCountersV2(moduleTracesCounter)
    val blockTime = Instant.parse("2021-01-01T00:00:00Z")
    val conflationEvents = mutableListOf<BlocksConflation>()
    conflationService.onConflatedBatch { conflationEvent: BlocksConflation ->
      conflationEvents.add(conflationEvent)
      SafeFuture.completedFuture(Unit)
    }
    val blockChunks = blocks.shuffled().chunked(numberOfBlocks / numberOfThreads)
    assertThat(blockChunks.size).isEqualTo(numberOfThreads)

    val executor = Executors.newFixedThreadPool(numberOfThreads)
    blockChunks.forEach { chunck ->
      executor.submit {
        chunck.forEach {
          conflationService.newBlock(
            it,
            BlockCounters(
              blockNumber = it.number.toULong(),
              blockTimestamp = blockTime,
              tracesCounters = fixedTracesCounters,
              blockRLPEncoded = ByteArray(0),
            ),
          )
        }
      }
    }

    Awaitility.waitAtMost(Duration.ofSeconds(30)).until {
      conflationEvents.size >= expectedConflations
    }
    executor.shutdown()

    var expectedNexStartBlockNumber: ULong = 1u
    assertThat(conflationEvents.size).isEqualTo(expectedConflations)
    conflationEvents.forEach { event ->
      assertThat(event.conflationResult.startBlockNumber).isEqualTo(expectedNexStartBlockNumber)
      expectedNexStartBlockNumber = event.conflationResult.endBlockNumber + 1u
    }
    assertThat(conflationService.blocksToConflate.toArray()).isEmpty()
  }

  @Test
  fun `if calculator fails, error is propagated`() {
    val moduleTracesCounter = 10u
    val fixedTracesCounters = fakeTracesCountersV2(moduleTracesCounter)
    val blockTime = Instant.parse("2021-01-01T00:00:00Z")

    val expectedException = RuntimeException("Calculator failed!")
    val failingConflationCalculator: TracesConflationCalculator = mock()
    whenever(failingConflationCalculator.newBlock(any())).thenThrow(expectedException)
    conflationService = ConflationServiceImpl(failingConflationCalculator, mock(defaultAnswer = RETURNS_DEEP_STUBS))
    val block = createBlock(number = 1UL, gasLimit = 20_000_000UL)

    assertThatThrownBy {
      conflationService.newBlock(
        block,
        BlockCounters(
          blockNumber = block.number.toULong(),
          blockTimestamp = blockTime,
          tracesCounters = fixedTracesCounters,
          blockRLPEncoded = ByteArray(0),
        ),
      )
    }.isEqualTo(expectedException)
  }
}
