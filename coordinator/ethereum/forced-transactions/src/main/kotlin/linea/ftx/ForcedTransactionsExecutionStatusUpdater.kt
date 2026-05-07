package linea.ftx

import linea.contract.events.ForcedTransactionAddedEvent
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.forcedtx.ForcedTransactionsClient
import linea.ftx.conflation.ForcedTransactionsSafeBlockNumberManager
import linea.ftx.conflation.FtxConflationInfo
import linea.persistence.ForcedTransactionRecord
import linea.persistence.ForcedTransactionsDao
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Queue
import kotlin.time.Clock
import kotlin.time.Duration

fun interface ForcedTransactionsProvider {
  fun getUnprocessedForcedTransactions(): SafeFuture<List<ForcedTransactionAddedEvent>>
}

fun interface ForcedTransactionProcessedListener {
  fun onFtxProcessed(ftxStatus: FtxConflationInfo)
}

/**
 * Responsible for getting Forced Transactions status from the sequencer and update local DB.
 * Ensures sequential processing without gaps: only processes FTX #N after all FTXs < N are processed.
 */
internal class ForcedTransactionsStatusUpdater(
  private val dao: ForcedTransactionsDao,
  private val ftxClient: ForcedTransactionsClient,
  private val safeBlockNumberManager: ForcedTransactionsSafeBlockNumberManager,
  private val ftxQueue: Queue<ForcedTransactionWithTimestamp>,
  private val ftxProcessedListener: ForcedTransactionProcessedListener,
  lastProcessedFtxNumber: ULong,
  private val ftxProcessingDelay: Duration = Duration.ZERO,
  private val clock: Clock,
  private val log: Logger = LogManager.getLogger(ForcedTransactionsStatusUpdater::class.java),
) : ForcedTransactionsProvider {

  @Volatile
  private var nextExpectedFtxNumber: ULong = lastProcessedFtxNumber + 1UL

  override fun getUnprocessedForcedTransactions(): SafeFuture<List<ForcedTransactionAddedEvent>> {
    return filterOutAlreadyProcessed(ftxQueue.toList())
      .thenApply {
        filterOutFtxOutsideDelayWindow(it)
          .map { it.event }
      }
      .thenPeek {
        // Only release the safe-block lock when the L1-events queue is genuinely drained.
        // The filtered list can be empty while the queue is not (e.g. entries inside the
        // ftxProcessingDelay window or awaiting their sequencer status), in which case
        // releasing would let conflation advance past an in-flight FTX block.
        if (it.isEmpty() && ftxQueue.isEmpty()) {
          safeBlockNumberManager.unprocessedFtxQueueIsEmpty()
        }
      }
  }

  private fun filterOutFtxOutsideDelayWindow(
    ftxs: List<ForcedTransactionWithTimestamp>,
  ): List<ForcedTransactionWithTimestamp> {
    val now = clock.now()
    return ftxs
      .filter { ftxWithTimestamp ->
        val timeElapsedSinceL1Submission = now - ftxWithTimestamp.l1BlockTimestamp
        val isReady = timeElapsedSinceL1Submission >= ftxProcessingDelay
        if (!isReady) {
          log.info(
            "ftx={} submittedAt={} can only be processed after enforced delay={} timeElapsedSinceL1Submission={}",
            ftxWithTimestamp.event.forcedTransactionNumber,
            ftxWithTimestamp.l1BlockTimestamp,
            ftxProcessingDelay,
            timeElapsedSinceL1Submission,
          )
        }
        isReady
      }
  }

  private fun filterOutAlreadyProcessed(
    ftxs: List<ForcedTransactionWithTimestamp>,
  ): SafeFuture<List<ForcedTransactionWithTimestamp>> {
    log.debug(
      "filtering FTXs: nextExpectedFtxNumber={}, queuedFtxs={}",
      nextExpectedFtxNumber,
      ftxs.map { it.event.forcedTransactionNumber },
    )

    // Sort by ftxNumber and only consider transactions >= expectedNum
    val sortedFtxs = ftxs
      .filter { it.event.forcedTransactionNumber >= nextExpectedFtxNumber }
      .sortedBy { it.event.forcedTransactionNumber }

    // Find consecutive transactions starting from expectedNum (no gaps)
    val consecutiveFtxs = mutableListOf<ForcedTransactionWithTimestamp>()
    var currentExpected = nextExpectedFtxNumber
    for (ftx in sortedFtxs) {
      if (ftx.event.forcedTransactionNumber == currentExpected) {
        consecutiveFtxs.add(ftx)
        currentExpected++
      } else {
        log.debug(
          "gap detected: expected={}, found={}, stopping consecutive check",
          currentExpected,
          ftx.event.forcedTransactionNumber,
        )
        break // Stop at first gap
      }
    }

    log.debug(
      "processing consecutive FTXs: {}",
      {
        consecutiveFtxs.map { it.event.forcedTransactionNumber }
      },
    )

    // Process consecutive transactions sequentially
    return processConsecutiveTransactions(consecutiveFtxs)
  }

  /**
   * Process transactions sequentially in order.
   * For each processed transaction, remove it from the queue.
   * Stop at the first unprocessed transaction.
   */
  private fun processConsecutiveTransactions(
    remaining: List<ForcedTransactionWithTimestamp>,
  ): SafeFuture<List<ForcedTransactionWithTimestamp>> {
    if (remaining.isEmpty()) {
      return SafeFuture.completedFuture(emptyList())
    }

    return processTransaction(remaining.first().event).thenCompose { wasProcessed ->
      if (wasProcessed) {
        processConsecutiveTransactions(remaining.drop(1))
      } else {
        SafeFuture.completedFuture(remaining)
      }
    }
  }

  private fun processTransaction(ftx: ForcedTransactionAddedEvent): SafeFuture<Boolean> {
    return findSequencerProcessingRecord(ftx).thenApply { dbRecord ->
      if (dbRecord != null) {
        // IMPORTANT: only remove from FTX queue if addFtxToConflation succeeds,
        // if conflation queue is full, addFtxToConflation fails and retries on next tick
        addFtxToConflation(dbRecord)
        // Remove from queue by matching the event's ftx number
        ftxQueue.removeIf { it.event.forcedTransactionNumber == ftx.forcedTransactionNumber }
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

  private fun addFtxToConflation(ftxDbRecord: ForcedTransactionRecord) {
    // IMPORTANT: notify calculators BEFORE releasing the safeBlockNumber lock.
    // ftxProcessedBySequencer may release the lock, allowing conflation to proceed
    // immediately. If the result isn't visible to the conflation calculator before
    // the lock release, it would miss the trigger.
    ftxProcessedListener.onFtxProcessed(
      FtxConflationInfo(
        ftxNumber = ftxDbRecord.ftxNumber,
        blockNumber = ftxDbRecord.simulatedExecutionBlockNumber,
        inclusionResult = ftxDbRecord.inclusionResult,
      ),
    )
    safeBlockNumberManager.ftxProcessedBySequencer(
      ftxNumber = ftxDbRecord.ftxNumber,
      simulatedExecutionBlockNumber = ftxDbRecord.simulatedExecutionBlockNumber,
    )
  }

  private fun findSequencerProcessingRecord(ftx: ForcedTransactionAddedEvent): SafeFuture<ForcedTransactionRecord?> {
    // 1. check local db, if not present, check sequencer and update DB if processed by sequencer
    return dao
      .findByNumber(ftxNumber = ftx.forcedTransactionNumber)
      .thenCompose { dbRecord ->
        if (dbRecord != null) {
          SafeFuture.completedFuture(dbRecord)
        } else {
          checkStatusAndUpdateLocalDb(ftx)
        }
      }
  }

  private fun checkStatusAndUpdateLocalDb(ftx: ForcedTransactionAddedEvent): SafeFuture<ForcedTransactionRecord?> {
    return ftxClient
      .lineaFindForcedTransactionStatus(ftx.forcedTransactionNumber)
      .thenCompose { ftxStatus ->
        if (ftxStatus == null) {
          SafeFuture.completedFuture(null)
        } else {
          if (ftxStatus.inclusionResult == ForcedTransactionInclusionResult.Phylax) {
            log.warn(
              "FTX rejected by Phylax: potential security incident, please notify Ops/Support immediately." +
                " Stopping conflation. sequencerResult={}",
              ftxStatus,
            )
            // we cannot proceed with ftx conflations, shall short circuit here!!
            SafeFuture.completedFuture(null)
          } else {
            log.info("ftx={} sequencerResult={}", ftx.forcedTransactionNumber, ftxStatus)
            val record = ForcedTransactionRecord(
              ftxNumber = ftx.forcedTransactionNumber,
              inclusionResult = ftxStatus.inclusionResult,
              simulatedExecutionBlockNumber = ftxStatus.blockNumber,
              simulatedExecutionBlockTimestamp = ftxStatus.blockTimestamp,
              ftxBlockNumberDeadline = ftx.blockNumberDeadline,
              ftxRollingHash = ftx.forcedTransactionRollingHash,
              ftxRlp = ftx.rlpEncodedSignedTransaction,
              proofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
            )
            dao
              .save(record)
              .thenApply { record }
          }
        }
      }
  }
}
