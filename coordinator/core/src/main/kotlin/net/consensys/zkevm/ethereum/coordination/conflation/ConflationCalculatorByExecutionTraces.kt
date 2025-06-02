package net.consensys.zkevm.ethereum.coordination.conflation

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import net.consensys.linea.metrics.Counter
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import net.consensys.linea.traces.TracesCounters
import net.consensys.linea.traces.TracingModule
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

class ConflationCalculatorByExecutionTraces(
  val tracesCountersLimit: TracesCounters,
  private val emptyTracesCounters: TracesCounters,
  metricsFacade: MetricsFacade,
  private val log: Logger = LogManager.getLogger(ConflationCalculatorByExecutionTraces::class.java),
) : ConflationCalculator {
  private val overflownTracesMetricsCounters = HashMap<TracingModule, Counter>().also {
    tracesCountersLimit.entries()
      .forEach { (module, _) ->
        it[module] = metricsFacade.createCounter(
          category = LineaMetricsCategory.CONFLATION,
          name = "overflow.evm",
          description = "Number of times ${module.name} traces counter has overflown",
          tags = listOf(
            Tag(
              key = "module",
              value = module.name,
            ),
          ),
        )
      }
  }

  override val id: String = ConflationTrigger.TRACES_LIMIT.name
  private var inprogressTracesCounters: TracesCounters = emptyTracesCounters

  override fun checkOverflow(blockCounters: BlockCounters): ConflationCalculator.OverflowTrigger? {
    return if (canAppendTraces(blockCounters.tracesCounters)) {
      return null
    } else {
      val tracesValidationResult = checkTracesAreWithinCaps(blockCounters.blockNumber, blockCounters.tracesCounters)
      if (tracesValidationResult is Err) {
        log.warn(tracesValidationResult.error)
        ConflationCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, true)
      } else {
        countOverflownTraces(blockCounters.tracesCounters)
        ConflationCalculator.OverflowTrigger(ConflationTrigger.TRACES_LIMIT, false)
      }
    }
  }

  override fun appendBlock(blockCounters: BlockCounters) {
    val appendResult = inprogressTracesCounters.add(blockCounters.tracesCounters)
    if (isOversizedBlockOnTopOfNonEmptyConflation(appendResult)) {
      // if single block overflows traces conflation limits,
      // we allow in because it will trigger conflation next and will be flushed
      throw IllegalStateException("Block ${blockCounters.blockNumber} overflows traces conflation limits.")
    } else {
      inprogressTracesCounters = appendResult
    }
  }

  private fun isOversizedBlockOnTopOfNonEmptyConflation(
    countersAfterConflation: TracesCounters,
  ): Boolean {
    return !countersAfterConflation.allTracesWithinLimits(tracesCountersLimit) &&
      inprogressTracesCounters != emptyTracesCounters
  }

  private fun canAppendTraces(tracesCounters: TracesCounters): Boolean {
    return inprogressTracesCounters.add(tracesCounters).allTracesWithinLimits(tracesCountersLimit)
  }

  override fun reset() {
    this.inprogressTracesCounters = emptyTracesCounters
  }

  override fun copyCountersTo(
    counters: ConflationCounters,
  ) {
    counters.tracesCounters = inprogressTracesCounters
  }

  private fun checkTracesAreWithinCaps(blockNumber: ULong, tracesCounters: TracesCounters): Result<Unit, String> {
    val overSizeTraces = tracesCounters.oversizedTraces(tracesCountersLimit)
    return if (overSizeTraces.isNotEmpty()) {
      val errorMessage = overSizeTraces.joinToString(
        separator = ", ",
        prefix = "oversized block: block=$blockNumber, oversize traces TRACE(count, limit, overflow): [",
        postfix = "]",
      ) { (moduleName, count, limit) ->
        "$moduleName($count, $limit, ${count - limit})"
      }
      countOverflownTraces(tracesCounters)
      Err(errorMessage)
    } else {
      Ok(Unit)
    }
  }

  private fun countOverflownTraces(tracesCounters: TracesCounters) {
    tracesCounters.entries().forEach { (moduleName, moduleCount) ->
      if (moduleCount + inprogressTracesCounters[moduleName] > tracesCountersLimit[moduleName]) {
        overflownTracesMetricsCounters[moduleName]?.increment()
      }
    }
  }
}
