package linea.ftx.conflation

import linea.forcedtx.ForcedTransactionInclusionResult
import linea.forcedtx.ForcedTransactionInclusionStatus
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.ethereum.coordination.aggregation.AggregationTrigger
import net.consensys.zkevm.ethereum.coordination.aggregation.AggregationTriggerCalculatorByTargetBlockNumbers
import net.consensys.zkevm.ethereum.coordination.aggregation.AggregationTriggerType
import net.consensys.zkevm.ethereum.coordination.aggregation.SyncAggregationTriggerCalculator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.Queue

/**
 * Synchronous aggregation trigger calculator that creates aggregation boundaries at FTX execution blocks
 * that require invalidity proofs.
 *
 * Consumes from processedFtxQueue (shared with ConflationCalculatorByForcedTransaction)
 * to detect processed FTXs with non-Included results and returns INVALIDITY_PROOF
 * aggregation triggers at (ftx.blockNumber - 1) to isolate the FTX block for invalidity proof generation.
 *
 * Rules:
 * - Trigger aggregation at (ftx.blockNumber - 1) when inclusionResult != Included
 * - Do NOT trigger if inclusionResult == Included (successfully executed FTXs)
 *
 * Note: This calculator is responsible for consuming items from the shared queue.
 * ConflationCalculatorByForcedTransaction reads from the queue without consuming.
 * Queue cleanup happens on reset() to ensure all FTXs are processed before clearing.
 *
 * This ensures that:
 * 1. Aggregations are sealed before failed FTX execution blocks
 * 2. Failed FTX blocks are isolated in their own aggregation for invalidity proof generation
 * 3. Successfully included FTXs don't create unnecessary aggregation boundaries
 */
class AggregationCalculatorByForcedTransaction(
  private val processedFtxQueue: Queue<ForcedTransactionInclusionStatus>,
  private val log: Logger = LogManager.getLogger(AggregationCalculatorByForcedTransaction::class.java),
) : SyncAggregationTriggerCalculator {

  // Track pending trigger blocks (blockNumber - 1) for non-included FTXs
  private val pendingTriggerBlocks = mutableSetOf<ULong>()

  // Delegate to AggregationTriggerCalculatorByTargetBlockNumbers for actual triggering logic
  private val delegateCalculator = AggregationTriggerCalculatorByTargetBlockNumbers(
    targetEndBlockNumbersProvider = { pendingTriggerBlocks.toList() },
    triggerType = AggregationTriggerType.FORCED_TRANSACTION,
    log = log,
  )

  @Synchronized
  private fun consumeProcessedFtxs() {
    // Consume all available FTX statuses from the queue
    val processedFtxs = mutableListOf<ForcedTransactionInclusionStatus>()
    while (true) {
      val ftx = processedFtxQueue.poll() ?: break
      processedFtxs.add(ftx)
    }

    if (processedFtxs.isEmpty()) {
      return
    }

    // Only add trigger blocks for FTXs that were NOT successfully included
    // Trigger at (blockNumber - 1) to seal aggregation before failed FTX execution block
    val newTriggerBlocks = processedFtxs
      .filter { ftx ->
        ftx.inclusionResult != ForcedTransactionInclusionResult.Included
      }
      .map { ftx ->
        // ftx.blockNumber is always greater than 0 (0 is genesis block),
        // In practice, greater than 2 most of the cases because network bootstrapping
        // takes a few blocks to deploy L2 protocol contracts
        ftx.blockNumber - 1UL
      }
      .toSet()

    pendingTriggerBlocks.addAll(newTriggerBlocks)
    logFtxPendingAggregation(processedFtxs, newTriggerBlocks)
  }

  fun logFtxPendingAggregation(processedFtxs: List<ForcedTransactionInclusionStatus>, newTriggerBlocks: Set<ULong>) {
    if (newTriggerBlocks.isNotEmpty()) {
      val failedFtxs = processedFtxs.filter { it.inclusionResult != ForcedTransactionInclusionResult.Included }
      log.info(
        "added {} FTX aggregation trigger blocks for {} non-included FTXs. ftxs={} total pending triggers: {}",
        newTriggerBlocks.size,
        failedFtxs.size,
        failedFtxs.map { "ftx=${it.ftxNumber} result=${it.inclusionResult}" },
        pendingTriggerBlocks.sorted(),
      )
    } else {
      log.debug(
        "processed {} FTXs, all were successfully included (no aggregation triggers needed)",
        processedFtxs.size,
      )
    }
  }

  @Synchronized
  override fun newBlob(blobCounters: BlobCounters) {
    // Delegate to the target block numbers calculator
    delegateCalculator.newBlob(blobCounters)
  }

  @Synchronized
  override fun checkAggregationTrigger(blobCounters: BlobCounters): AggregationTrigger? {
    // First, consume all available processed FTXs from the queue
    consumeProcessedFtxs()
    // Delegate to the target block numbers calculator
    val trigger = delegateCalculator.checkAggregationTrigger(blobCounters)

    if (trigger != null) {
      log.info(
        "FTX aggregation trigger detected: sealing aggregation at block {} (before failed FTX execution at block {})",
        blobCounters.endBlockNumber,
        blobCounters.endBlockNumber + 1UL,
      )
      // Remove the trigger block after it's been used
      pendingTriggerBlocks.remove(blobCounters.endBlockNumber)
    }

    return trigger
  }

  @Synchronized
  override fun reset() {
    // Consume any remaining FTXs from queue
    consumeProcessedFtxs()

    // DO NOT clear pendingTriggerBlocks - they represent FTXs that still need to be processed
    // Triggers persist across aggregation resets until their corresponding blocks are reached

    // Reset delegate to start tracking a new aggregation
    delegateCalculator.reset()

    log.trace("FTX aggregation calculator reset")
  }
}
