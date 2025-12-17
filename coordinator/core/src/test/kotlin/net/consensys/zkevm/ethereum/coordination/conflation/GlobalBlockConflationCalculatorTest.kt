package net.consensys.zkevm.ethereum.coordination.conflation

import kotlinx.datetime.Instant
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.fakeTracesCountersV2
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.invocation.InvocationOnMock
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.inOrder
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

class GlobalBlockConflationCalculatorTest {
  private lateinit var calculatorByDealine: DeferredTriggerConflationCalculator
  private lateinit var calculatorByData: ConflationCalculator
  private lateinit var calculatorByTraces: ConflationCalculator
  private lateinit var globalCalculator: GlobalBlockConflationCalculator
  private val lastBlockNumber: ULong = 0uL
  private val fakeCountersAfterConflation = fakeTracesCountersV2(123u)
  private val fakeDataSizeAfterConflation = 123u
  val block1Counters =
    BlockCounters(
      blockNumber = 1uL,
      blockTimestamp = Instant.parse("2023-12-11T00:00:00.000Z"),
      tracesCounters = fakeTracesCountersV2(10u),
      blockRLPEncoded = ByteArray(0),
    )
  val block2Counters =
    BlockCounters(
      blockNumber = 2uL,
      blockTimestamp = Instant.parse("2023-12-11T00:00:02.000Z"),
      tracesCounters = fakeTracesCountersV2(20u),
      blockRLPEncoded = ByteArray(0),
    )

  private lateinit var conflations: MutableList<ConflationCalculationResult>

  @BeforeEach
  fun beforeEach() {
    val fakeUpdater = { invocationMock: InvocationOnMock ->
      val inflightCounters = invocationMock.arguments[0] as ConflationCounters
      // val blockCounters = invocationMock.arguments[1] as BlockCounters?
      inflightCounters.tracesCounters = fakeCountersAfterConflation
      inflightCounters.dataSize = fakeDataSizeAfterConflation
      Unit
    }
    calculatorByDealine =
      mock<DeferredTriggerConflationCalculator> {
        on { id }.thenAnswer { "TIME_LIMIT" }
        on { copyCountersTo(any<ConflationCounters>()) }
          .thenAnswer(fakeUpdater)
      }
    calculatorByTraces =
      mock<ConflationCalculator> {
        on { id }.thenAnswer { "TRACES_LIMIT" }
        on { copyCountersTo(any<ConflationCounters>()) }
          .thenAnswer(fakeUpdater)
      }
    calculatorByData =
      mock<ConflationCalculator> {
        on { id }.thenAnswer { "DATA_LIMIT" }
        on { copyCountersTo(any<ConflationCounters>()) }
          .thenAnswer(fakeUpdater)
      }

    globalCalculator =
      GlobalBlockConflationCalculator(
        lastBlockNumber = lastBlockNumber,
        syncCalculators = listOf(calculatorByTraces, calculatorByData),
        deferredTriggerConflationCalculators = listOf(calculatorByDealine),
        emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
      )
    conflations = mutableListOf<ConflationCalculationResult>()
    globalCalculator.onConflatedBatch { trigger ->
      conflations.add(trigger)
      SafeFuture.completedFuture(Unit)
    }
  }

  @Test
  fun `should not allow duplicated calculators`() {
    assertThatThrownBy {
      GlobalBlockConflationCalculator(
        lastBlockNumber = lastBlockNumber,
        syncCalculators = listOf(calculatorByTraces, calculatorByData, calculatorByDealine),
        deferredTriggerConflationCalculators = listOf(calculatorByDealine),
        emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("calculators must not contain duplicates")
  }

  @Test
  fun `should not accept blocks out of order`() {
    assertThatThrownBy { globalCalculator.newBlock(block2Counters) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("Blocks to conflate must be sequential: lastBlockNumber=0, new blockNumber=2")
  }

  @Test
  fun `when a calculator returns TriggerOverflow, it should trigger conflation listener`() {
    whenever(calculatorByData.checkOverflow(eq(block2Counters)))
      .thenAnswer { ConflationCalculator.OverflowTrigger(ConflationTrigger.DATA_LIMIT, singleBlockOverSized = false) }

    globalCalculator.newBlock(block1Counters)
    globalCalculator.newBlock(block2Counters)

    assertThat(conflations).isEqualTo(
      listOf(
        ConflationCalculationResult(
          startBlockNumber = 1uL,
          endBlockNumber = 1uL,
          conflationTrigger = ConflationTrigger.DATA_LIMIT,
          tracesCounters = fakeCountersAfterConflation,
        ),
      ),
    )

    calculatorByData.inOrder {
      verify().checkOverflow(block1Counters)
      verify().appendBlock(block1Counters)
      verify().checkOverflow(block2Counters)
      verify().reset()
      verify().appendBlock(block2Counters)
    }
    calculatorByTraces.inOrder {
      verify().checkOverflow(block1Counters)
      verify().appendBlock(block1Counters)
      verify().checkOverflow(block2Counters)
      verify().reset()
      verify().appendBlock(block2Counters)
    }
    calculatorByDealine.inOrder {
      verify().checkOverflow(block1Counters)
      verify().appendBlock(block1Counters)
      verify().checkOverflow(block2Counters)
      verify().reset()
      verify().appendBlock(block2Counters)
    }
  }

  @Test
  fun `when a async calculator triggers, should trigger conflation listener`() {
    globalCalculator.newBlock(block1Counters)
    globalCalculator.newBlock(block2Counters)

    globalCalculator.handleConflationTrigger(ConflationTrigger.TIME_LIMIT)

    assertThat(conflations).isEqualTo(
      listOf(
        ConflationCalculationResult(
          startBlockNumber = 1uL,
          endBlockNumber = 2uL,
          conflationTrigger = ConflationTrigger.TIME_LIMIT,
          tracesCounters = fakeCountersAfterConflation,
        ),
      ),
    )

    calculatorByData.inOrder {
      verify().checkOverflow(block1Counters)
      verify().appendBlock(block1Counters)
      verify().checkOverflow(block2Counters)
      verify().appendBlock(block2Counters)
      verify().reset()
    }
    calculatorByTraces.inOrder {
      verify().checkOverflow(block1Counters)
      verify().appendBlock(block1Counters)
      verify().checkOverflow(block2Counters)
      verify().appendBlock(block2Counters)
      verify().reset()
    }
    calculatorByDealine.inOrder {
      verify().checkOverflow(block1Counters)
      verify().appendBlock(block1Counters)
      verify().checkOverflow(block2Counters)
      verify().appendBlock(block2Counters)
      verify().reset()
    }
  }
}
