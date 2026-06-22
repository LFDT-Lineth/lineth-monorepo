package linea.coordinator.app.conflationbacktesting

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.concurrent.atomics.AtomicBoolean
import kotlin.concurrent.atomics.AtomicLong
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.concurrent.atomics.fetchAndUpdate

/**
 * Tracks per-flow progress (execution, compression, aggregation) for a conflation backtesting job.
 *
 * Each flow submits its proof request asynchronously and independently:
 *  - Execution proof request creation depends on a remote traces conflation API call with retries.
 *  - Compression proof request creation depends only on blob creation in the conflation calculator.
 *  - Aggregation proof request creation depends on compression proof requests being submitted.
 *
 * Because these flows run independently, an aggregation request for the target end block can be
 * created while earlier execution proof requests are still retrying. Backtesting is complete only
 * when every flow has reached [targetEndBlockNumber].
 */
@OptIn(ExperimentalAtomicApi::class)
class BacktestingProgressTracker(
  startBlockNumber: ULong,
  private val targetEndBlockNumber: ULong,
  private val log: Logger = LogManager.getLogger(BacktestingProgressTracker::class.java),
) {
  // Each flow's "no progress yet" state is the block before the backtesting range starts.
  // BlockCreationMonitor reads this baseline via lastExecutionRequestEndBlock() to gate how
  // far ahead of the proven head it may fetch; initialising below startBlockNumber - 1 would
  // keep the gap above blocksFetchLimit and stall block fetching forever.
  private val initialEndBlock: Long = startBlockNumber.toLong() - 1L
  private val lastExecutionEndBlock = AtomicLong(initialEndBlock)
  private val lastCompressionEndBlock = AtomicLong(initialEndBlock)
  private val lastAggregationEndBlock = AtomicLong(initialEndBlock)
  private val completionLogged = AtomicBoolean(false)

  fun recordExecutionRequestEndBlock(endBlockNumber: ULong) {
    record("execution", lastExecutionEndBlock, endBlockNumber)
  }

  fun recordCompressionRequestEndBlock(endBlockNumber: ULong) {
    record("compression", lastCompressionEndBlock, endBlockNumber)
  }

  fun recordAggregationRequestEndBlock(endBlockNumber: ULong) {
    record("aggregation", lastAggregationEndBlock, endBlockNumber)
  }

  fun lastExecutionRequestEndBlock(): Long = lastExecutionEndBlock.load()

  fun lastCompressionRequestEndBlock(): Long = lastCompressionEndBlock.load()

  fun lastAggregationRequestEndBlock(): Long = lastAggregationEndBlock.load()

  fun isComplete(): Boolean {
    val target = targetEndBlockNumber.toLong()
    return lastExecutionEndBlock.load() >= target &&
      lastCompressionEndBlock.load() >= target &&
      lastAggregationEndBlock.load() >= target
  }

  private fun record(flowName: String, tracker: AtomicLong, endBlockNumber: ULong) {
    val newValue = endBlockNumber.toLong()
    tracker.fetchAndUpdate { current -> if (current < newValue) newValue else current }
    log.info(
      "Backtesting {} progress: lastEndBlock={} targetEndBlock={}",
      flowName,
      tracker.load(),
      targetEndBlockNumber,
    )
    if (isComplete() && completionLogged.compareAndSet(expectedValue = false, newValue = true)) {
      log.info("Conflation backtesting complete: targetEndBlock={}", targetEndBlockNumber)
    }
  }
}
