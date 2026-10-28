package lineth.conflation.calculators
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import linea.domain.BlockCounters
import linea.domain.ConflationTrigger
import lineth.conflation.ZERO_COINBASE
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.traces.TracesCounters
import net.consensys.linea.traces.TracesCountersV5
import net.consensys.linea.traces.TracingModuleV5
import net.consensys.linea.traces.fakeTracesCountersV5
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import kotlin.time.Instant

class ConflationTriggerCalculatorByExecutionTracesTest {
  private val tracesLimit = fakeTracesCountersV5(100u)
  private val testMeterRegistry = SimpleMeterRegistry()
  private val calculator =
    ConflationTriggerCalculatorByExecutionTraces(
      tracesCountersLimit = tracesLimit,
      metricsFacade = MicrometerMetricsFacade(testMeterRegistry, "test"),
    )
  private lateinit var conflationTriggerConsumer: ConflationTriggerConsumer

  @BeforeEach
  fun beforeEach() {
    conflationTriggerConsumer = mock<ConflationTriggerConsumer>()
  }

  private fun assertCountersEqualTo(expectedTracesCounters: TracesCounters) {
    val inflightCounters = ConflationCounters.empty(TracesCountersV5.EMPTY_TRACES_COUNT)
    calculator.copyCountersTo(inflightCounters)
    assertThat(inflightCounters)
      .isEqualTo(ConflationCounters(tracesCounters = expectedTracesCounters))
  }

  @Test
  fun `appendBlock should accumulate counters`() {
    calculator.appendBlock(blockCounters(fakeTracesCountersV5(10u)))
    assertCountersEqualTo(fakeTracesCountersV5(10u))

    calculator.appendBlock(blockCounters(fakeTracesCountersV5(20u)))
    assertCountersEqualTo(fakeTracesCountersV5(30u))

    calculator.appendBlock(blockCounters(fakeTracesCountersV5(40u)))
    assertCountersEqualTo(fakeTracesCountersV5(70u))

    calculator.reset()
    assertCountersEqualTo(fakeTracesCountersV5(0u))
  }

  @Test
  fun `appendBlock should throw if counter go over limit when accumulated`() {
    calculator.appendBlock(blockCounters(fakeTracesCountersV5(10u)))
    assertThatThrownBy { calculator.appendBlock(blockCounters(fakeTracesCountersV5(91u))) }
      .isInstanceOf(IllegalStateException::class.java)

    // it should allow single oversized block
    calculator.reset()
    calculator.appendBlock(blockCounters(fakeTracesCountersV5(200u)))
  }

  @Test
  fun `copyCountersTo`() {
    val inflightConflationCounters = ConflationCounters.empty(TracesCountersV5.EMPTY_TRACES_COUNT)
    calculator.appendBlock(blockCounters(fakeTracesCountersV5(10u)))
    calculator.copyCountersTo(inflightConflationCounters)
    assertThat(inflightConflationCounters)
      .isEqualTo(ConflationCounters(tracesCounters = fakeTracesCountersV5(10u)))

    calculator.appendBlock(blockCounters(fakeTracesCountersV5(20u)))
    calculator.copyCountersTo(inflightConflationCounters)
    assertThat(inflightConflationCounters)
      .isEqualTo(ConflationCounters(tracesCounters = fakeTracesCountersV5(30u)))

    calculator.appendBlock(blockCounters(fakeTracesCountersV5(30u)))
    calculator.copyCountersTo(inflightConflationCounters)
    assertThat(inflightConflationCounters)
      .isEqualTo(ConflationCounters(tracesCounters = fakeTracesCountersV5(60u)))

    calculator.reset()
    calculator.copyCountersTo(inflightConflationCounters)
    assertThat(inflightConflationCounters)
      .isEqualTo(ConflationCounters(tracesCounters = fakeTracesCountersV5(0u)))
  }

  @Test
  fun `checkOverflow should return trigger when block is oversized`() {
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV5(100u)))).isNull()
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV5(101u))))
      .isEqualTo(ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, true))
  }

  @Test
  fun `checkOverflow should return trigger accumulated traces overflow`() {
    calculator.appendBlock(blockCounters(fakeTracesCountersV5(10u)))
    calculator.appendBlock(blockCounters(fakeTracesCountersV5(89u)))
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV5(1u)))).isNull()
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV5(2u))))
      .isEqualTo(ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, false))
  }

  @Test
  fun `module counters incremented when traces overflow`() {
    val overflowingTraces =
      listOf(
        TracingModuleV5.MMU,
        TracingModuleV5.ADD,
        TracingModuleV5.RLP_TXN,
      )
    val oversizedTraceCounters =
      TracesCountersV5(
        TracingModuleV5.entries.associate {
          if (overflowingTraces.contains(it)) {
            it to 101u
          } else {
            it to 0u
          }
        },
      )

    TracingModuleV5.entries.forEach { module ->
      val moduleOverflowCounter =
        testMeterRegistry.get("test.conflation.overflow.evm")
          .tag("module", module.name).counter()
      assertThat(moduleOverflowCounter.count()).isEqualTo(0.0)
    }
    assertThat(calculator.checkOverflow(blockCounters(fakeTracesCountersV5(100u)))).isNull()
    assertThat(calculator.checkOverflow(blockCounters(oversizedTraceCounters)))
      .isEqualTo(ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, true))

    TracingModuleV5.entries.forEach { module ->
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
      TracesCountersV5(
        TracingModuleV5.entries.associate {
          if (overflowingTraces.contains(it)) {
            it to 99u
          } else {
            it to 0u
          }
        },
      )

    calculator.appendBlock(blockCounters(fakeTracesCountersV5(10u)))
    assertThat(calculator.checkOverflow(blockCounters(overflowCounters)))
      .isEqualTo(ConflationTriggerCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, false))

    TracingModuleV5.entries.forEach { module ->
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
      coinbase = ZERO_COINBASE,
    )
  }
}
