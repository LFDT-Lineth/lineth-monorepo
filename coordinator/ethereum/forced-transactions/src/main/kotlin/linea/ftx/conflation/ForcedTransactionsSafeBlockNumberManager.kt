package linea.ftx.conflation

import linea.contract.events.ForcedTransactionAddedEvent
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

/**
 * Tracks the highest safe block number (inclusive) that can be conflated.
 *
 * Lock-release decisions are split across two methods so each carries a single
 * piece of state:
 *  - [caughtUpWithChainHeadAfterStartUp] only marks the L1 startup scan as finished;
 *  - [unprocessedFtxQueueIsEmpty] performs the actual release, and must only be
 *    invoked by the caller once the L1-events queue is genuinely drained
 *    (otherwise conflation could advance past the L2 block where an in-flight
 *    FTX will eventually execute).
 */
internal class ForcedTransactionsSafeBlockNumberManager(
  private val listener: SafeBlockNumberUpdateListener,
) {
  private val log: Logger = LogManager.getLogger(ForcedTransactionsSafeBlockNumberManager::class.java)
  private var safeBlockNumber: ULong? = 0UL
  private var startUpScanFinished: Boolean = false
  private val ftxInSequencerForProcessing: MutableList<ULong> = mutableListOf()

  private fun updateSafeBlockNumber(value: ULong?) {
    if (safeBlockNumber != value) {
      if (value == null) {
        log.info("releasing safeBlockNumber lock: safeBlockNumber={} --> null", safeBlockNumber)
      }
      safeBlockNumber = value
      listener.onSafeBlockNumberUpdate(safeBlockNumber)
    }
  }

  @Synchronized
  fun lockSafeBlockNumberBeforeSendingToSequencer(headBlockNumber: ULong) {
    if (safeBlockNumber != null && safeBlockNumber!! > 0UL) {
      log.info(
        "conflation already locked at safeBlockNumber={}, will not lock block={}",
        safeBlockNumber,
        headBlockNumber,
      )
      return
    }

    log.info("locking conflation at l2 blockNumber={}", headBlockNumber)
    updateSafeBlockNumber(headBlockNumber)
  }

  @Synchronized
  fun ftxSentToSequencer(ftx: List<ForcedTransactionAddedEvent>) {
    if (safeBlockNumber == null || safeBlockNumber == 0UL) {
      throw IllegalStateException("Safe Block Number lock should have been acquired before sending FTXs to sequencer")
    }
    this.ftxInSequencerForProcessing.addAll(ftx.map { it.forcedTransactionNumber })
  }

  /**
   * Called either on:
   * 1. start-up: with the latest ftx seen on the DB;
   * 2. runtime: when sequencer processes a ftx
   */
  @Synchronized
  fun ftxProcessedBySequencer(ftxNumber: ULong, simulatedExecutionBlockNumber: ULong) {
    if (safeBlockNumber != null && simulatedExecutionBlockNumber < safeBlockNumber!!) {
      throw IllegalStateException(
        "ftx=$ftxNumber simulatedExecutionBlockNumber=$simulatedExecutionBlockNumber " +
          "must be greater than or equal to safeBlockNumber=$safeBlockNumber",
      )
    }

    this.ftxInSequencerForProcessing.removeIf { it <= ftxNumber }
    if (ftxInSequencerForProcessing.isEmpty() && startUpScanFinished) {
      log.info(
        "all ftx sent to sequencer were processed, releasing lock: safeBlockNumber={} --> null",
        safeBlockNumber,
      )
      updateSafeBlockNumber(null)
    } else {
      log.info(
        "updating safeBlockNumber={}-->{} by processed ftx={} at blockNumber={}, " +
          "ftx in the sequencer {}",
        safeBlockNumber,
        simulatedExecutionBlockNumber,
        ftxNumber,
        simulatedExecutionBlockNumber,
        ftxInSequencerForProcessing,
      )
      updateSafeBlockNumber(simulatedExecutionBlockNumber)
    }
  }

  @Synchronized
  fun unprocessedFtxQueueIsEmpty() {
    // not safe to release lock until we finish start up L1 scan
    if (!startUpScanFinished) {
      return
    }
    updateSafeBlockNumber(null)
  }

  /**
   * Marks the L1 startup scan as finished. It does not release the lock on its own:
   * the L1 fetcher catching up only proves there are no further L1 events to discover,
   * not that the sequencer has confirmed every FTX already queued. The caller of
   * [unprocessedFtxQueueIsEmpty] is responsible for the actual release.
   * Idempotent.
   */
  @Synchronized
  fun caughtUpWithChainHeadAfterStartUp() {
    if (startUpScanFinished) {
      return
    }
    startUpScanFinished = true
    log.info("L1 startup scan finished; safeBlockNumber={}", safeBlockNumber)
  }

  @Synchronized
  fun forcedTransactionsUnsupportedYetByL1Contract() {
    log.info(
      "releasing Safe Block Number lock after startup: " +
        "contract version does not support forced transactions yet",
    )
    updateSafeBlockNumber(null)
  }
}
