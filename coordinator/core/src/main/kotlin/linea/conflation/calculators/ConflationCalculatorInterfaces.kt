package linea.conflation.calculators

import linea.conflation.BlobCreationHandler
import linea.domain.BlockCounters
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import net.consensys.linea.traces.TracesCounters
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class ConflationCounters(
  var dataSize: UInt = 0u,
  var blockCount: UInt = 0u,
  var tracesCounters: TracesCounters,
) {
  companion object {
    internal fun empty(emptyTracesCounters: TracesCounters): ConflationCounters {
      return ConflationCounters(tracesCounters = emptyTracesCounters)
    }
  }
}

interface ConflationTriggerCalculator {
  val id: String

  data class OverflowTrigger(
    val trigger: ConflationTrigger,
    val singleBlockOverSized: Boolean,
  )

  /**
   * Checks if the block can be appended.
   * If the block overflows the limits returns the conflation trigger otherwise null.
   */
  fun checkOverflow(blockCounters: BlockCounters): OverflowTrigger?

  /**
   * Accumulates block counters to current conflation.
   * If block would overflow the limits, it will throw an exception.
   */
  fun appendBlock(blockCounters: BlockCounters)

  fun reset()

  fun copyCountersTo(counters: ConflationCounters)
}

interface DeferredConflationTriggerCalculator : ConflationTriggerCalculator {
  fun setConflationTriggerConsumer(conflationTriggerConsumer: ConflationTriggerConsumer)
}

@FunctionalInterface
fun interface ConflationTriggerConsumer {
  fun handleConflationTrigger(trigger: ConflationTrigger)

  companion object {
    internal val noopConsumer: ConflationTriggerConsumer =
      object : ConflationTriggerConsumer {
        override fun handleConflationTrigger(trigger: ConflationTrigger) {}
      }
  }
}

interface BlockConflationCalculator {
  val lastBlockNumber: ULong

  fun newBlock(blockCounters: BlockCounters)

  fun onConflatedBatch(conflationConsumer: (ConflationCalculationResult) -> SafeFuture<*>)

  fun onBlobCreation(blobHandler: BlobCreationHandler)
}
