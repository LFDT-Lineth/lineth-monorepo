package linea.ftx

import linea.contract.events.ForcedTransactionAddedEvent
import linea.forcedtx.ForcedTransactionsClient
import linea.persistence.ftx.ForcedTransactionsDao
import net.consensys.zkevm.domain.ForcedTransactionRecord
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Queue

fun interface ForcedTransactionsProvider {
  fun getUnprocessedForcedTransactions(): SafeFuture<List<ForcedTransactionAddedEvent>>
}

/**
 * Responsible for getting Forced Transactions status from the sequencer and update local DB.
 * Ensures sequential processing without gaps: only processes FTX #N after all FTXs < N are processed.
 */
class ForcedTransactionsStatusUpdater(
  private val dao: ForcedTransactionsDao,
  private val ftxClient: ForcedTransactionsClient,
  private val ftxQueue: Queue<ForcedTransactionAddedEvent>,
  lastProcessedFtxNumber: ULong,
  private val log: Logger = LogManager.getLogger(ForcedTransactionsStatusUpdater::class.java),
) : ForcedTransactionsProvider {

  @Volatile
  private var nextExpectedFtxNumber: ULong = lastProcessedFtxNumber + 1UL

  override fun getUnprocessedForcedTransactions(): SafeFuture<List<ForcedTransactionAddedEvent>> {
    return filterOutAlreadyProcessed(ftxQueue.toList())
  }

  private fun filterOutAlreadyProcessed(
    ftxs: List<ForcedTransactionAddedEvent>,
  ): SafeFuture<List<ForcedTransactionAddedEvent>> {
    log.debug(
      "filtering FTXs: nextExpectedFtxNumber={}, queuedFtxs={}",
      nextExpectedFtxNumber,
      ftxs.map { it.forcedTransactionNumber },
    )

    // Sort by ftxNumber and only consider transactions >= expectedNum
    val sortedFtxs = ftxs
      .filter { it.forcedTransactionNumber >= nextExpectedFtxNumber }
      .sortedBy { it.forcedTransactionNumber }

    // Find consecutive transactions starting from expectedNum (no gaps)
    val consecutiveFtxs = mutableListOf<ForcedTransactionAddedEvent>()
    var currentExpected = nextExpectedFtxNumber
    for (ftx in sortedFtxs) {
      if (ftx.forcedTransactionNumber == currentExpected) {
        consecutiveFtxs.add(ftx)
        currentExpected++
      } else {
        log.debug(
          "Gap detected: expected={}, found={}, stopping consecutive check",
          currentExpected,
          ftx.forcedTransactionNumber,
        )
        break // Stop at first gap
      }
    }

    log.debug("Processing consecutive FTXs: {}", consecutiveFtxs.map { it.forcedTransactionNumber })

    // Process consecutive transactions sequentially
    return processConsecutiveTransactions(consecutiveFtxs)
  }

  /**
   * Process transactions sequentially in order.
   * For each processed transaction, remove it from the queue.
   * Stop at the first unprocessed transaction.
   */
  fun processConsecutiveTransactions(
    remaining: List<ForcedTransactionAddedEvent>,
  ): SafeFuture<List<ForcedTransactionAddedEvent>> {
    if (remaining.isEmpty()) {
      return SafeFuture.completedFuture(emptyList())
    }

    return processTransaction(remaining.first()).thenCompose { wasProcessed ->
      if (wasProcessed) {
        processConsecutiveTransactions(remaining.drop(1))
      } else {
        SafeFuture.completedFuture(remaining)
      }
    }
  }

  private fun processTransaction(ftx: ForcedTransactionAddedEvent): SafeFuture<Boolean> {
    return isAlreadyProcessed(ftx).thenApply { alreadyProcessed ->
      if (alreadyProcessed) {
        ftxQueue.remove(ftx)
        nextExpectedFtxNumber = ftx.forcedTransactionNumber + 1uL
        log.debug(
          "FTX #{} already processed, removed from queue. Next expected: {}",
          ftx.forcedTransactionNumber,
          nextExpectedFtxNumber,
        )
        true
      } else {
        log.debug(
          "FTX #{} not yet processed, stopping sequential check",
          ftx.forcedTransactionNumber,
        )
        false
      }
    }
  }

  private fun isAlreadyProcessed(ftx: ForcedTransactionAddedEvent): SafeFuture<Boolean> {
    // 1. check local db, if not present, check sequencer and update DB if processed by sequencer
    return dao
      .findByNumber(ftxNumber = ftx.forcedTransactionNumber)
      .thenCompose { dbRecord ->
        if (dbRecord != null) {
          SafeFuture.completedFuture(true)
        } else {
          checkStatusAndUpdateLocalDb(ftx)
        }
      }
  }

  private fun checkStatusAndUpdateLocalDb(ftx: ForcedTransactionAddedEvent): SafeFuture<Boolean> {
    return ftxClient
      .lineaFindForcedTransactionStatus(ftx.forcedTransactionNumber)
      .thenCompose { ftxStatus ->
        if (ftxStatus == null) {
          SafeFuture.completedFuture(false)
        } else {
          val record = ForcedTransactionRecord(
            ftxNumber = ftx.forcedTransactionNumber,
            inclusionResult = ftxStatus.inclusionResult,
            simulatedExecutionBlockNumber = ftxStatus.blockNumber,
            simulatedExecutionBlockTimestamp = ftxStatus.blockTimestamp,
          )
          dao
            .save(record)
            .thenApply { true }
        }
      }
  }
}
