package linea.ftx

import linea.contract.l1.LineaRollupSmartContractClientReadOnlyFinalizedStateProvider
import linea.domain.BlockParameter
import linea.persistence.ftx.ForcedTransactionsDao
import net.consensys.zkevm.domain.ForcedTransactionRecord
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.math.max

fun interface ForcedTransactionsResumePointProvider {
  fun getLastProcessedForcedTransactionNumber(): SafeFuture<ULong>
}

internal class ForcedTransactionsResumePointProviderImpl(
  val finalizedStateProvider: LineaRollupSmartContractClientReadOnlyFinalizedStateProvider,
  val l1HighestBlock: BlockParameter,
  val ftxDao: ForcedTransactionsDao,
  val log: Logger = LogManager.getLogger(ForcedTransactionsResumePointProvider::class.java),
) : ForcedTransactionsResumePointProvider {
  fun getLastProcessedForcedTransaction(): SafeFuture<Pair<ULong, ForcedTransactionRecord?>> {
    return finalizedStateProvider
      .getLatestFinalizedState(blockParameter = l1HighestBlock)
      .thenCombine(ftxDao.findHighestForcedTransaction()) { finalizedState, ftxRecord ->
        val highestProcessed = max(finalizedState.forcedTransactionNumber, ftxRecord?.ftxNumber ?: 0UL)
        if (highestProcessed == ftxRecord?.ftxNumber) {
          Pair(highestProcessed, ftxRecord)
        } else {
          Pair(highestProcessed, null)
        }
      }
      .exceptionally { th ->
        if (th is UnsupportedOperationException || th.cause is UnsupportedOperationException) {
          // contract is still before V8 or no finalization event yet, let's default to ftx0
          log.info(
            "failed to get finalized forcedTransactionNumber, " +
              "will default forcedTransactionNumber=0, errorMessage={}",
            th.message,
          )
          Pair(0UL, null)
        } else {
          throw th
        }
      }
  }

  override fun getLastProcessedForcedTransactionNumber(): SafeFuture<ULong> {
    return getLastProcessedForcedTransaction().thenApply { it.first }
  }
}
