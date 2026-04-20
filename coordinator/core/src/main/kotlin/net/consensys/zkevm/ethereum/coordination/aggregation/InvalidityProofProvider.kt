package net.consensys.zkevm.ethereum.coordination.aggregation

import linea.domain.InvalidityProofIndex
import linea.persistence.ForcedTransactionsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface InvalidityProofProvider {
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
