package linea.ftx.conflation

import linea.contract.events.ForcedTransactionAddedEvent
import net.consensys.zkevm.ethereum.coordination.blockcreation.ConflationSafeBlockNumberProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

/**
 * Provides Safe Block Number (SBN) based on Forced Transactions in flight.
 *
 * SBN = min(simulatedExecutionBlockNumber) - 1 across all FTX records.
 * Returns null when no FTX records exist (unrestricted).
 *
 * Caches the result with periodic refresh to avoid frequent database queries.
 */

internal class ForcedTransactionsSafeBlockNumberManager : ConflationSafeBlockNumberProvider {
  private val log: Logger = LogManager.getLogger(ForcedTransactionsSafeBlockNumberManager::class.java)
  private var safeBlockNumber: ULong? = 0UL
  private var startUpScanFinished: Boolean = false
  private val ftxInSequencerForProcessing: MutableList<ULong> = mutableListOf()

  @Synchronized
  override fun getHighestSafeBlockNumber(): ULong? = safeBlockNumber

  /**
   * Called either on:
   * 1. start-up: with the latest ftx seen on the DB;
   * 2. runtime: when sequencer processes a ftx
   */
  @Synchronized
  fun ftxProcessedBySequencer(ftxNumber: ULong, simulatedExecutionBlockNumber: ULong) {
    if (safeBlockNumber == null) {
      throw IllegalStateException("Safe Block Number lock should have been acquired before sending FTXs to sequencer")
    }
    if (simulatedExecutionBlockNumber < safeBlockNumber!!) {
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

    this.ftxInSequencerForProcessing.removeIf { it <= ftxNumber }
    if (ftxInSequencerForProcessing.isEmpty() && startUpScanFinished) {
      log.info("all ftx sent to sequencer were processed, releasing Safe Block Number lock")
      safeBlockNumber = null
    } else {
      safeBlockNumber = simulatedExecutionBlockNumber
      log.info(
        "ftx={} processed at safeBlockNumber={}, ftx in the sequencer {}",
        ftxNumber,
        safeBlockNumber,
        ftxInSequencerForProcessing,
      )
    }
  }

  @Synchronized
  fun ftxSentToSequencer(ftx: List<ForcedTransactionAddedEvent>) {
    if (safeBlockNumber == null || safeBlockNumber == 0UL) {
      throw IllegalStateException("Safe Block Number lock should have been acquired before sending FTXs to sequencer")
    }
    this.ftxInSequencerForProcessing.addAll(ftx.map { it.forcedTransactionNumber })
  }

  @Synchronized
  fun lockSafeBlockNumberBeforeSendingToSequencer(headBlockNumber: ULong) {
    if (safeBlockNumber != null && safeBlockNumber!! > 0UL) {
      log.info(
        "conflation already locked at safeBlockNumber={}, will no lock block={}",
        safeBlockNumber,
        headBlockNumber,
      )
      return
    }

    log.info("locking conflation at latest l2 blockNumber={}", headBlockNumber)
    safeBlockNumber = headBlockNumber
  }

  @Synchronized
  fun unprocessedFtxQueueIsEmpty() {
    // not safe to release lock until we finish start up L1 scan
    if (!startUpScanFinished) {
      return
    }
    log.info("releasing Safe Block Number lock")
    safeBlockNumber = null
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
      safeBlockNumber = null
    }
  }

  @Synchronized
  fun forcedTransactionsUnsupportedYetByL1Contract() {
    log.info(
      "releasing Safe Block Number lock after startup: " +
        "contract version does not support forced transactions yet",
    )
    safeBlockNumber = null
  }
}
