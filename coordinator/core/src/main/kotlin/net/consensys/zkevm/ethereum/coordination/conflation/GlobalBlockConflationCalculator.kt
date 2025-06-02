package net.consensys.zkevm.ethereum.coordination.conflation

import net.consensys.linea.traces.TracesCounters
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

internal val NOOP_CONSUMER: (ConflationCalculationResult) -> SafeFuture<*> =
  { _: ConflationCalculationResult ->
    SafeFuture.completedFuture(Unit)
  }

internal data class InflightConflation(
  var startBlockNumber: ULong?,
  var counters: ConflationCounters,
) {

  fun toConflationResult(endBlockNumber: ULong, trigger: ConflationTrigger): ConflationCalculationResult {
    return ConflationCalculationResult(
      startBlockNumber = startBlockNumber!!,
      endBlockNumber = endBlockNumber,
      conflationTrigger = trigger,
      tracesCounters = counters.tracesCounters,
    )
  }

  companion object {
    fun empty(emptyTracesCounters: TracesCounters): InflightConflation {
      return InflightConflation(
        startBlockNumber = null,
        counters = ConflationCounters.empty(emptyTracesCounters),
      )
    }
  }
}

class GlobalBlockConflationCalculator(
  override var lastBlockNumber: ULong,
  syncCalculators: List<ConflationCalculator>,
  deferredTriggerConflationCalculators: List<DeferredTriggerConflationCalculator>,
  private val emptyTracesCounters: TracesCounters,
  private val log: Logger = LogManager.getLogger(GlobalBlockConflationCalculator::class.java),
) : TracesConflationCalculator, ConflationTriggerConsumer {
  private var conflationConsumer: (ConflationCalculationResult) -> SafeFuture<*> = NOOP_CONSUMER
  private var inflightConflation: InflightConflation = InflightConflation.empty(emptyTracesCounters)
  private var calculators: List<ConflationCalculator> = syncCalculators + deferredTriggerConflationCalculators

  init {
    require(calculators.isNotEmpty()) { "calculators must not be empty" }
    require(calculators.size == calculators.distinctBy { it.id }.size) {
      "calculators must not contain duplicates"
    }
    deferredTriggerConflationCalculators.forEach { it.setConflationTriggerConsumer(this) }
  }

  @Synchronized
  override fun newBlock(blockCounters: BlockCounters) {
    ensureBlockIsInOrder(blockCounters.blockNumber)
    conflateBlock(blockCounters)
  }

  private fun isCurrentConflationEmpty(): Boolean {
    return inflightConflation.startBlockNumber == null
  }

  private fun conflateBlock(blockCounters: BlockCounters) {
    log.trace(
      "conflatingBlock: blockNumber={} startBlockNumber={} accCounters={}",
      blockCounters.blockNumber,
      inflightConflation.startBlockNumber,
      inflightConflation.counters,
    )
    val triggers = calculators.mapNotNull {
      val overflowTrigger = it.checkOverflow(blockCounters)
      log.trace("CHECK: calculator=${it.id}, blockNumber=${blockCounters.blockNumber}, trigger=$overflowTrigger")
      overflowTrigger
    }.sortedBy { it.trigger.triggerPriority }

    if (triggers.isNotEmpty()) {
      // we have at least one trigger. Need to flush current conflation and start new one
      log.trace("TRIGGERS: triggers={} isCurrentConflationEmpty={}", triggers, isCurrentConflationEmpty())
      val conflationTrigger = triggers.first().trigger
      if (!isCurrentConflationEmpty()) {
        // we have to flush current conflation
        fireConflationAndResetState(lastBlockNumber, conflationTrigger)
      }
    }

    // can be appended to current conflation
    if (isCurrentConflationEmpty()) {
      inflightConflation.startBlockNumber = blockCounters.blockNumber
    }
    calculators.forEach {
      log.trace("APPENDING: calculator=${it.id}, blockNumber=${blockCounters.blockNumber}")
      it.appendBlock(blockCounters)
      it.copyCountersTo(inflightConflation.counters)
    }

    lastBlockNumber = blockCounters.blockNumber
    log.trace(
      "conflatingBlock FINISH: blockNumber={} startBlockNumber={} accCounters={}",
      blockCounters.blockNumber,
      inflightConflation.startBlockNumber,
      inflightConflation.counters,
    )
  }

  private fun fireConflationAndResetState(endBlockNumber: ULong, conflationTrigger: ConflationTrigger) {
    calculators.forEach { it.copyCountersTo(inflightConflation.counters) }
    val conflationResult = inflightConflation.toConflationResult(
      endBlockNumber = endBlockNumber,
      trigger = conflationTrigger,
    )
    log.trace("conflationTrigger: trigger=$conflationTrigger, result=$conflationResult")
    conflationConsumer.invoke(conflationResult)
    reset()
  }

  @Synchronized
  override fun handleConflationTrigger(trigger: ConflationTrigger) {
    fireConflationAndResetState(lastBlockNumber, trigger)
  }

  @Synchronized
  override fun onConflatedBatch(conflationConsumer: (ConflationCalculationResult) -> SafeFuture<*>) {
    if (this.conflationConsumer != NOOP_CONSUMER) {
      throw IllegalStateException("Consumer is already set")
    }
    this.conflationConsumer = conflationConsumer
  }

  @Synchronized
  override fun onBlobCreation(blobHandler: BlobCreationHandler) {
    TODO("Not yet implemented")
  }

  @Synchronized
  internal fun getConflationInProgress(): InflightConflation = inflightConflation

  @Synchronized
  fun reset() {
    calculators.forEach(ConflationCalculator::reset)
    inflightConflation = InflightConflation.empty(emptyTracesCounters)
  }

  private fun ensureBlockIsInOrder(blockNumber: ULong) {
    if (blockNumber != (lastBlockNumber + 1u)) {
      val error = IllegalArgumentException(
        "Blocks to conflate must be sequential: lastBlockNumber=$lastBlockNumber, new blockNumber=$blockNumber",
      )
      log.error(error.message)
      throw error
    }
  }
}
