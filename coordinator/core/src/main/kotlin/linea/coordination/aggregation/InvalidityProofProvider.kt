package linea.coordination.aggregation

import linea.domain.InvalidityProofIndex
import linea.persistence.ForcedTransactionsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface InvalidityProofProvider {
  /**
   * Returns invalidity-proof indexes for the forced transaction (if any) that opens an
   * aggregation.
   *
   * Invariant: a forced transaction is always the first transaction of its execution block, and
   * that block triggers a conflation cut. The FTX's `simulatedExecutionBlockNumber` is therefore
   * always equal to the starting block of the aggregation that contains it — an aggregation
   * holds at most one FTX, and it sits on the start boundary.
   *
   * Given that invariant, the DAO query is narrowed by passing
   * [aggregationStartingBlockNumber] as the upper bound on `simulatedExecutionBlockNumber`.
   * Combined with [ftxStartingNumber] (which excludes FTXs already covered by parent
   * aggregations), the result is exactly the FTX that opens this aggregation, or empty if it
   * has none.
   *
   * @param ftxStartingNumber the lowest FTX number to include (inclusive); FTXs strictly before
   *   this one have already been finalised in a parent aggregation.
   * @param aggregationStartingBlockNumber the L2 block number that starts this aggregation —
   *   equal to the simulated execution block of the FTX that opens it, if any.
   */
  fun getInvalidityProofs(
    ftxStartingNumber: ULong,
    aggregationStartingBlockNumber: ULong,
  ): SafeFuture<List<InvalidityProofIndex>>
}

class InvalidityProofProviderImpl(
  private val forcedTransactionsDao: ForcedTransactionsDao,
) : InvalidityProofProvider {
  override fun getInvalidityProofs(
    ftxStartingNumber: ULong,
    aggregationStartingBlockNumber: ULong,
  ): SafeFuture<List<InvalidityProofIndex>> {
    return forcedTransactionsDao
      .findByStartingNumber(
        ftxStartingNumberInclusive = ftxStartingNumber,
        endSimulatedExecutionBlockNumberInclusive = aggregationStartingBlockNumber,
      ).thenApply { forcedTransactions ->
        forcedTransactions.map { forcedTransaction ->
          InvalidityProofIndex(
            ftxNumber = forcedTransaction.ftxNumber,
            simulatedExecutionBlockNumber = forcedTransaction.simulatedExecutionBlockNumber,
            startBlockTimestamp = forcedTransaction.simulatedExecutionBlockTimestamp,
          )
        }
      }
  }
}
