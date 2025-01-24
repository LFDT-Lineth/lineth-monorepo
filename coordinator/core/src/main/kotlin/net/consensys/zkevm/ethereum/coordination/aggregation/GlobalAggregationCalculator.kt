package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.linea.metrics.Counter
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.PriorityQueue

class GlobalAggregationCalculator(
  private var lastBlockNumber: ULong,
  private val syncAggregationTrigger: List<SyncAggregationTriggerCalculator>,
  private val deferredAggregationTrigger: List<DeferredAggregationTriggerCalculator>,
  metricsFacade: MetricsFacade,
  private val aggregationSizeMultipleOf: UInt
) : AggregationCalculator {
  private var aggregationHandler: AggregationHandler = AggregationHandler.NOOP_HANDLER

  private val log: Logger = LogManager.getLogger(this::class.java)
  private val pendingBlobs = PriorityQueue<BlobCounters> { b1, b2 ->
    b1.startBlockNumber.compareTo(b2.startBlockNumber)
  }

  private val blobsCounter: Counter = metricsFacade.createCounter(
    category = LineaMetricsCategory.AGGREGATION,
    name = "calculator.blobs.accepted",
    description = "Number of blobs accepted by aggregation calculator"
  )
  private val batchesCounter: Counter = metricsFacade.createCounter(
    category = LineaMetricsCategory.AGGREGATION,
    name = "calculator.batches.accepted",
    description = "Number of batches accepted by aggregation calculator"
  )

  init {
    require(syncAggregationTrigger.size + deferredAggregationTrigger.size > 0) { "Specify at least one trigger" }
    deferredAggregationTrigger.forEach { it.onAggregationTrigger(this::handleAggregationTrigger) }
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "proofs.ready",
      description = "Number of proofs pending for aggregation",
      measurementSupplier = {
        pendingBlobs.size + pendingBlobs.sumOf { it.numberOfBatches }.toLong()
      }
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "batches.ready",
      description = "Number of batches pending for aggregation",
      measurementSupplier = {
        pendingBlobs.sumOf { it.numberOfBatches }.toLong()
      }
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.AGGREGATION,
      name = "blobs.ready",
      description = "Number of blobs pending for aggregation",
      measurementSupplier = {
        pendingBlobs.size
      }
    )
  }

  private fun processBlob(blobCounters: BlobCounters) {
    log.trace("Processing blob={}", blobCounters.intervalString())

    pendingBlobs.add(blobCounters)
    val aggregationTriggers =
      syncAggregationTrigger.mapNotNull { it.checkAggregationTrigger(blobCounters) }

    if (aggregationTriggers.isNotEmpty()) {
      val aggregationTrigger = aggregationTriggers.first()
      handleAggregationTrigger(aggregationTrigger)
    } else {
      // Only add new blob to the trigger calculators if there was no aggregation trigger.
      // If there was an aggregation trigger this blob will be reprocessed after handling the aggregation trigger.
      syncAggregationTrigger.forEach { it.newBlob(blobCounters) }
      deferredAggregationTrigger.forEach { it.newBlob(blobCounters) }
    }
  }

  @Synchronized
  override fun newBlob(blobCounters: BlobCounters) {
    log.trace("New blob={}", blobCounters.intervalString())

    ensureBlobIsInOrder(blobCounters.startBlockNumber)
    lastBlockNumber = blobCounters.endBlockNumber
    blobsCounter.increment()
    batchesCounter.increment(blobCounters.numberOfBatches.toDouble())
    processBlob(blobCounters)
  }

  override fun onAggregation(aggregationHandler: AggregationHandler) {
    this.aggregationHandler = aggregationHandler
  }

  @Synchronized
  internal fun handleAggregationTrigger(aggregationTrigger: AggregationTrigger) {
    log.info("Aggregation Triggered aggregationTrigger={}", aggregationTrigger)

    val aggregation = aggregationTrigger.aggregation

    if (pendingBlobs.isEmpty() ||
      pendingBlobs.find { it.startBlockNumber == aggregation.startBlockNumber } == null ||
      pendingBlobs.find { it.endBlockNumber == aggregation.endBlockNumber } == null
    ) {
      val exception = IllegalStateException(
        "Aggregation triggered when pending blobs do not contain blobs within aggregation interval. " +
          "aggregationTrigger=$aggregationTrigger " +
          "pendingBlobs=${pendingBlobs.map { it.intervalString() }}"
      )
      log.error(exception.message, exception)
      throw exception
    }

    val blobsInAggregation = mutableListOf<BlobCounters>()
    while (pendingBlobs.isNotEmpty() &&
      pendingBlobs.peek().startBlockNumber >= aggregation.startBlockNumber &&
      pendingBlobs.peek().endBlockNumber <= aggregation.endBlockNumber
    ) {
      blobsInAggregation.add(pendingBlobs.poll())
    }

    val blobsNotInAggregation = mutableListOf<BlobCounters>()
    while (pendingBlobs.isNotEmpty()) {
      blobsNotInAggregation.add(pendingBlobs.poll())
    }

    val updatedAggregationSize = getUpdatedAggregationSize(
      blobsInAggregation.size.toUInt(),
      aggregationSizeMultipleOf
    )

    val blobsInUpdatedAggregation = blobsInAggregation.subList(0, updatedAggregationSize.toInt())
    val blobsNotInUpdatedAggregation = blobsInAggregation.subList(
      updatedAggregationSize.toInt(),
      blobsInAggregation.size
    )

    val updatedAggregation = BlobsToAggregate(
      startBlockNumber = blobsInUpdatedAggregation.first().startBlockNumber,
      endBlockNumber = blobsInUpdatedAggregation.last().endBlockNumber
    )

    log.info(
      "aggregation: trigger={} aggregation={} updatedAggregation={} " +
        "blobsCount={} batchesCount={} blobs={} aggregationSizeMultiple={}",
      aggregationTrigger.aggregationTriggerType.name,
      aggregation.intervalString(),
      updatedAggregation.intervalString(),
      blobsInUpdatedAggregation.size,
      blobsInUpdatedAggregation.sumOf { it.numberOfBatches },
      blobsInUpdatedAggregation.map { it.intervalString() },
      aggregationSizeMultipleOf
    )

    // Reset the trigger calculators now that we have a valid aggregation to handle
    resetTriggerCalculators()

    aggregationHandler.onAggregation(updatedAggregation)

    // Reprocess the remaining blobs only after handling the current aggregation as the reprocessing can trigger
    // another aggregation
    val blobsToReprocess = (blobsNotInUpdatedAggregation + blobsNotInAggregation).sortedBy { it.startBlockNumber }

    blobsToReprocess.forEach {
      log.trace("Reprocessing blob={}", it.intervalString())
      processBlob(it)
    }
  }

  private fun resetTriggerCalculators() {
    log.trace("Reset on GlobalAggregationCalculator")
    syncAggregationTrigger.forEach { it.reset() }
    deferredAggregationTrigger.forEach { it.reset() }
  }

  private fun ensureBlobIsInOrder(blockNumber: ULong) {
    if (blockNumber != (lastBlockNumber + 1u)) {
      val error = IllegalArgumentException(
        "Blobs to aggregate must be sequential: lastBlockNumber=$lastBlockNumber, startBlockNumber=$blockNumber for " +
          "new blob"
      )
      log.error(error.message)
      throw error
    }
  }

  companion object {
    fun getUpdatedAggregationSize(aggregationSize: UInt, aggregationSizeMultipleOf: UInt): UInt {
      return if (aggregationSize > aggregationSizeMultipleOf) {
        aggregationSize - (aggregationSize % aggregationSizeMultipleOf)
      } else {
        aggregationSize
      }
    }
  }
}
