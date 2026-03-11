package linea.ftx.conflation

/**
 * Provides Safe Block Number (SBN) based on Forced Transactions in flight.
 *
 * SBN = min(simulatedExecutionBlockNumber) - 1 across all FTX records.
 * Returns null when no FTX records exist (unrestricted).
 *
 * Caches the result with periodic refresh to avoid frequent database queries.
 */

fun interface SafeBlockNumberUpdateListener {
  fun onSafeBlockNumberUpdate(safeBlockNumber: ULong?)
}
