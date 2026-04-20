package linea.persistence

import tech.pegasys.teku.infrastructure.async.SafeFuture

interface ForcedTransactionsDao {
  fun save(ftx: ForcedTransactionRecord): SafeFuture<Unit>
  fun findByNumber(ftxNumber: ULong): SafeFuture<ForcedTransactionRecord?>
  fun list(): SafeFuture<List<ForcedTransactionRecord>>
  fun deleteFtxUpToInclusive(ftxNumber: ULong): SafeFuture<Int>
  fun findHighestForcedTransaction(
    upToSimulatedExecutionBlockNumberInclusive: ULong? = null,
  ): SafeFuture<ForcedTransactionRecord?> {
    return list()
      .thenApply { allFtxs ->
        allFtxs
          .let { ftxs ->
            upToSimulatedExecutionBlockNumberInclusive
              ?.let { endBlock -> ftxs.filter { ftx -> ftx.simulatedExecutionBlockNumber <= endBlock } }
              ?: ftxs
          }
          .maxByOrNull { ftx -> ftx.ftxNumber }
      }
  }

  fun findByStartingNumber(
    ftxStartingNumberInclusive: ULong,
    endSimulatedExecutionBlockNumberInclusive: ULong,
  ): SafeFuture<List<ForcedTransactionRecord>> {
    return list().thenApply { allFtx ->
      allFtx.filter { ftx ->
        ftx.ftxNumber >= ftxStartingNumberInclusive &&
          ftx.simulatedExecutionBlockNumber <= endSimulatedExecutionBlockNumberInclusive
      }
    }
  }
}

/**
 * Forced transactions are an Opt-in feature in Linea stack,
 * so we provide a no-op implementation of the DAO that can be used when the feature is disabled.
 *
 * Also, until this feature is fully implemented and integrated, this no-op implementation can be used
 * to avoid the need for a real database setup in tests that don't require it.
 */
class DisabledForcedTransactionsDao : ForcedTransactionsDao {
  override fun save(ftx: ForcedTransactionRecord): SafeFuture<Unit> =
    SafeFuture.failedFuture(IllegalStateException("ForcedTransactions persistence is disabled"))
  override fun findByNumber(ftxNumber: ULong): SafeFuture<ForcedTransactionRecord?> = SafeFuture.completedFuture(null)
  override fun list(): SafeFuture<List<ForcedTransactionRecord>> = SafeFuture.completedFuture(emptyList())
  override fun deleteFtxUpToInclusive(ftxNumber: ULong): SafeFuture<Int> = SafeFuture.completedFuture(0)
}
