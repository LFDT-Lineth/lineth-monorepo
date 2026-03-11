package linea.ftx.conflation

import linea.contract.events.ForcedTransactionAddedEvent
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

internal class ForcedTransactionsSafeBlockNumberManager(
  private val listener: SafeBlockNumberUpdateListener,
) {
  private val log: Logger = LogManager.getLogger(ForcedTransactionsSafeBlockNumberManager::class.java)
  private var safeBlockNumber: ULong? = 0UL
  private var startUpScanFinished: Boolean = false
  private val ftxInSequencerForProcessing: MutableList<ULong> = mutableListOf()

  private fun updateSafeBlockNumber(value: ULong?) {
    safeBlockNumber = value
    listener.onSafeBlockNumberUpdate(safeBlockNumber)
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
        "simulatedExecutionBlockNumber must be greater than or equal to safeBlockNumber" +
          "simulatedExecutionBlockNumber=$simulatedExecutionBlockNumber, safeBlockNumber=$safeBlockNumber",
      )
    }
    log.info(
      "locking conflation: ftxNumber={} at blockNumber={}",
      ftxNumber,
      simulatedExecutionBlockNumber,
    )
    safeBlockNumber = simulatedExecutionBlockNumber

    this.ftxInSequencerForProcessing.removeIf { it <= ftxNumber }
    if (ftxInSequencerForProcessing.isEmpty() && startUpScanFinished) {
      log.info("all ftx sent to sequencer were processed, releasing Safe Block Number lock")
      updateSafeBlockNumber(null)
    } else {
      log.info(
        "ftx={} processed at safeBlockNumber={}, ftx in the sequencer {}",
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
    log.info("releasing Safe Block Number lock")
    updateSafeBlockNumber(null)
  }

  /**
   * Releases Safe Block Number lock after startup
   * it's idempotent
   */
  @Synchronized
  fun caughtUpWithChainHeadAfterStartUp() {
    if (startUpScanFinished) {
      return
    }
    startUpScanFinished = true
    if (this.ftxInSequencerForProcessing.isNotEmpty()) {
      return
    }
    if (safeBlockNumber == 0UL) {
      // still locked at 0 from start, no events on L1, release lock
      log.info("releasing Safe Block Number lock after startup")
      updateSafeBlockNumber(null)
    }
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
