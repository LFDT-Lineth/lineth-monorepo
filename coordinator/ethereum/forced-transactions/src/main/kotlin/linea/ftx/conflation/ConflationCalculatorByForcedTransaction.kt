package linea.ftx.conflation

import linea.forcedtx.ForcedTransactionInclusionResult
import linea.forcedtx.ForcedTransactionInclusionStatus
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCounters
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.util.Queue
import kotlin.collections.map

/**
 * Deferred trigger calculator that creates conflation boundaries at FTX execution blocks
 * that require invalidity proofs.
 *
 * Reads from processedFtxQueue (shared with AggregationCalculatorByForcedTransaction)
 * to detect processed FTXs with non-Included results and triggers FORCED_TRANSACTION
 * conflation at (blockNumber - 1) to isolate the FTX block for invalidity proof generation.
 *
 * Rules:
 * - Trigger conflation at (ftx.blockNumber - 1) when inclusionResult != Included
 * - Do NOT trigger if inclusionResult == Included (successfully executed FTXs)
 *
 * Note: This calculator reads from the queue without consuming.
 * AggregationCalculatorByForcedTransaction is responsible for consuming items.
 *
 * This ensures that:
 * 1. Failed FTXs are isolated in their own conflation for invalidity proof generation
 * 2. Successfully included FTXs don't create unnecessary conflation boundaries
 * 3. The conflation before the failed FTX block is sealed at (blockNumber - 1)
 */
class ConflationCalculatorByForcedTransaction(
  private val processedFtxQueue: Queue<ForcedTransactionInclusionStatus>,
  private val log: Logger = LogManager.getLogger(ConflationCalculatorByForcedTransaction::class.java),
) : ConflationCalculator {
  override val id: String = "FTX_INVALIDITY_PROOF_CONFLATION"

  // Track pending trigger blocks (blockNumber - 1) for non-included FTXs
  private val pendingTriggerBlocks = mutableSetOf<ULong>()

  @Volatile
  private var lastBlockNumber: ULong = 0UL

  @Synchronized
  override fun checkOverflow(blockCounters: BlockCounters): ConflationCalculator.OverflowTrigger? {
    // First, read all available processed FTXs from the queue (without consuming)
    readProcessedFtxs()

    // Check if this block should trigger conflation
    return if (pendingTriggerBlocks.contains(blockCounters.blockNumber)) {
      log.info(
        "FTX conflation overflow detected at block {} (will seal before FTX execution block {})",
        blockCounters.blockNumber,
        blockCounters.blockNumber + 1UL,
      )
      ConflationCalculator.OverflowTrigger(
        trigger = ConflationTrigger.FORCED_TRANSACTION,
        singleBlockOverSized = false,
      )
    } else {
      null
    }
  }

  private fun readProcessedFtxs() {
    // Read all available FTX statuses from the queue WITHOUT consuming
    // they cannot be consumed, it's AggregationCalculatorByForcedTransaction responsibility to consume them
    val processedFtxs = processedFtxQueue.toList()

    if (processedFtxs.isEmpty()) {
      return
    }

    // Only add trigger blocks for FTXs that were NOT successfully included
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

    // Only update if we have new trigger blocks not already tracked
    val actuallyNewBlocks = newTriggerBlocks - pendingTriggerBlocks
    pendingTriggerBlocks.addAll(actuallyNewBlocks)
    logNewFtxConflationTriggers(actuallyNewBlocks, processedFtxs)
  }

  fun logNewFtxConflationTriggers(
    actuallyNewBlocks: Set<ULong>,
    processedFtxs: List<ForcedTransactionInclusionStatus>,
  ) {
    if (actuallyNewBlocks.isNotEmpty()) {
      val newFailedFtxs = processedFtxs.filter {
        it.inclusionResult != ForcedTransactionInclusionResult.Included &&
          actuallyNewBlocks.contains(if (it.blockNumber > 0UL) it.blockNumber - 1UL else null)
      }
      log.info(
        "added {} new FTX conflation trigger blocks for {} non-included FTXs. ftxs={} total pending triggers: {}",
        actuallyNewBlocks.size,
        newFailedFtxs.size,
        newFailedFtxs.map { "ftx=${it.ftxNumber} result=${it.inclusionResult}" },
        pendingTriggerBlocks.sorted(),
      )
    }
  }

  @Synchronized
  override fun appendBlock(blockCounters: BlockCounters) {
    pendingTriggerBlocks.removeIf { it <= blockCounters.blockNumber }
    lastBlockNumber = blockCounters.blockNumber
  }

  @Synchronized
  override fun reset() {
    // Don't clear pendingTriggerBlocks on reset - they persist until processed
    log.trace("FTX conflation calculator reset at block {}", lastBlockNumber)
  }

  override fun copyCountersTo(counters: ConflationCounters) {
    // No counters to copy
  }
}
