package linea.ftx

import linea.contract.l1.LineaRollupSmartContractClientReadOnlyFinalizedStateProvider
import linea.domain.BlockParameter
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface ForcedTransactionsResumePointProvider {
  fun getLastProcessedForcedTransactionNumber(): SafeFuture<ULong>
}

class ForcedTransactionsResumePointProviderImpl(
  val finalizedStateProvider: LineaRollupSmartContractClientReadOnlyFinalizedStateProvider,
  val l1HighestBlock: BlockParameter,
  val log: Logger = LogManager.getLogger(ForcedTransactionsResumePointProvider::class.java),
) : ForcedTransactionsResumePointProvider {
  override fun getLastProcessedForcedTransactionNumber(): SafeFuture<ULong> {
    return finalizedStateProvider
      .getLatestFinalizedState(blockParameter = l1HighestBlock)
      .thenApply { finalizedState -> finalizedState.forcedTransactionNumber }
      .exceptionally { th ->
        if (th is UnsupportedOperationException || th.cause is UnsupportedOperationException) {
          // contract is still before V8 or no finalization event yet, let's default to ftx0
          log.info(
            "failed to get finalized forcedTransactionNumber, " +
              "will default forcedTransactionNumber=0, errorMessage={}",
            th.message,
          )
          0UL
        } else {
          throw th
        }
      }
  }
}
