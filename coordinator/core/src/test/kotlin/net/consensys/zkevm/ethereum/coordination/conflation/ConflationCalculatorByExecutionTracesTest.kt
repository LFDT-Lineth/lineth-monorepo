package net.consensys.zkevm.ethereum.coordination.conflation

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.traces.TracesCounters
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.TracingModuleV2
import net.consensys.linea.traces.fakeTracesCountersV2
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import kotlin.time.Instant

class ConflationCalculatorByExecutionTracesTest {
  private val tracesLimit = fakeTracesCountersV2(100u)
  private val testMeterRegistry = SimpleMeterRegistry()
  private val calculator =
    ConflationCalculatorByExecutionTraces(
      tracesLimit,
      TracesCountersV2.EMPTY_TRACES_COUNT,
      metricsFacade = MicrometerMetricsFacade(testMeterRegistry, "test"),
    )
  private lateinit var conflationTriggerConsumer: ConflationTriggerConsumer

  @BeforeEach
  fun beforeEach() {
    conflationTriggerConsumer = mock<ConflationTriggerConsumer>()
  }

  private fun assertCountersEqualTo(expectedTracesCounters: TracesCounters) {
    val inflightCounters = ConflationCounters.empty(TracesCountersV2.EMPTY_TRACES_COUNT)
    calculator.copyCountersTo(inflightCounters)
    assertThat(inflightCounters)
      .isEqualTo(ConflationCounters(tracesCounters = expectedTracesCounters))
  }

  @Test
  fun `appendBlock should accumulate counters`() {
    calculator.appendBlock(blockCounters(fakeTracesCountersV2(10u)))
    assertCountersEqualTo(fakeTracesCountersV2(10u))

    calculator.appendBlock(blockCounters(fakeTracesCountersV2(20u)))
    assertCountersEqualTo(fakeTracesCountersV2(30u))

    calculator.appendBlock(blockCounters(fakeTracesCountersV2(40u)))
    assertCountersEqualTo(fakeTracesCountersV2(70u))

    calculator.reset()
    assertCountersEqualTo(fakeTracesCountersV2(0u))
  }

  @Test
  fun `appendBlock should throw if counter go over limit when accumulated`() {
    calculator.appendBlock(blockCounters(fakeTracesCountersV2(10u)))
    assertThatThrownBy { calculator.appendBlock(blockCounters(fakeTracesCountersV2(91u))) }
      .isInstanceOf(IllegalStateException::class.java)

    // it should allow single oversized block
    calculator.reset()
    calculator.appendBlock(blockCounters(fakeTracesCountersV2(200u)))
  }

  @Test
  fun `copyCountersTo`() {
    val inflightConflationCounters = ConflationCounters.empty(TracesCountersV2.EMPTY_TRACES_COUNT)
    calculator.appendBlock(blockCounters(fakeTracesCountersV2(10u)))
    calculator.copyCountersTo(inflightConflationCounters)
    assertThat(inflightConflationCounters)
      .isEqualTo(ConflationCounters(tracesCounters = fakeTracesCountersV2(10u)))

    calculator.appendBlock(blockCounters(fakeTracesCountersV2(20u)))
    calculator.copyCountersTo(inflightConflationCounters)
    assertThat(inflightConflationCounters)
      .isEqualTo(ConflationCounters(tracesCounters = fakeTracesCountersV2(30u)))

    calculator.appendBlock(blockCounters(fakeTracesCountersV2(30u)))
    calculator.copyCountersTo(inflightConflationCounters)
    assertThat(inflightConflationCounters)
      .isEqualTo(ConflationCounters(tracesCounters = fakeTracesCountersV2(60u)))

    calculator.reset()
    calculator.copyCountersTo(inflightConflationCounters)
    assertThat(inflightConflationCounters)
      .isEqualTo(ConflationCounters(tracesCounters = fakeTracesCountersV2(0u)))
  }

  @Test
  fun `checkOverflow should return trigger when block is oversized`() {
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV2(100u)))).isNull()
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV2(101u))))
      .isEqualTo(ConflationCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, true))
  }

  @Test
  fun `checkOverflow should return trigger accumulated traces overflow`() {
    calculator.appendBlock(blockCounters(fakeTracesCountersV2(10u)))
    calculator.appendBlock(blockCounters(fakeTracesCountersV2(89u)))
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV2(1u)))).isNull()
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV2(2u))))
      .isEqualTo(ConflationCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, false))
  }

  @Test
  fun `module counters incremented when traces overflow`() {
    val overflowingTraces =
      listOf(
        TracingModuleV2.MMU,
        TracingModuleV2.ADD,
        TracingModuleV2.RLP_TXN,
      )
    val oversizedTraceCounters =
      TracesCountersV2(
        TracingModuleV2.entries.associate {
          if (overflowingTraces.contains(it)) {
            it to 101u
          } else {
            it to 0u
          }
        },
      )

    TracingModuleV2.entries.forEach { module ->
      val moduleOverflowCounter =
        testMeterRegistry.get("test.conflation.overflow.evm")
          .tag("module", module.name).counter()
      assertThat(moduleOverflowCounter.count()).isEqualTo(0.0)
    }
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV2(100u)))).isNull()
    assertThat(calculator.checkOverflow(blockCounters(oversizedTraceCounters)))
      .isEqualTo(ConflationCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, true))

    TracingModuleV2.entries.forEach { module ->
      val moduleOverflowCounter =
        testMeterRegistry.get("test.conflation.overflow.evm")
          .tag("module", module.name).counter()

      if (overflowingTraces.contains(module)) {
        assertThat(moduleOverflowCounter.count()).isEqualTo(1.0)
      } else {
        assertThat(moduleOverflowCounter.count()).isEqualTo(0.0)
      }
    }

    val overflowCounters =
      TracesCountersV2(
        TracingModuleV2.entries.associate {
          if (overflowingTraces.contains(it)) {
            it to 99u
          } else {
            it to 0u
          }
        },
      )

    calculator.appendBlock(blockCounters(fakeTracesCountersV2(10u)))
    assertThat(calculator.checkOverflow(blockCounters(overflowCounters)))
      .isEqualTo(ConflationCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, false))

    TracingModuleV2.entries.forEach { module ->
      val moduleOverflowCounter =
        testMeterRegistry.get("test.conflation.overflow.evm")
          .tag("module", module.name).counter()

      if (overflowingTraces.contains(module)) {
        assertThat(moduleOverflowCounter.count()).isEqualTo(2.0)
      } else {
        assertThat(moduleOverflowCounter.count()).isEqualTo(0.0)
      }
    }
  }

  private fun blockCounters(tracesCounters: TracesCounters, blockNumber: ULong = 1uL): BlockCounters {
    return BlockCounters(
      blockNumber = blockNumber,
      blockTimestamp = Instant.parse("2021-01-01T00:00:00Z"),
      tracesCounters = tracesCounters,
      blockRLPEncoded = ByteArray(0),
    )
  }
}
