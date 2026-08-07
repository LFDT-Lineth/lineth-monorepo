package lineth.transactionexclusion.service

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import lineth.error.DuplicatedRecordException
import lineth.metrics.MetricsFacade
import lineth.transactionexclusion.ErrorType
import lineth.transactionexclusion.RejectedTransaction
import lineth.transactionexclusion.TransactionExclusionError
import lineth.transactionexclusion.TransactionExclusionServiceV1
import lineth.transactionexclusion.TransactionExclusionServiceV1.SaveRejectedTransactionStatus
import lineth.transactionexclusion.metrics.LineaMetricsCategory
import lineth.zkevm.persistence.dao.rejectedtransaction.RejectedTransactionsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Duration

class TransactionExclusionServiceV1Impl(
  private val config: Config,
  private val repository: RejectedTransactionsDao,
  metricsFacade: MetricsFacade,
  private val clock: Clock = Clock.System,
) : TransactionExclusionServiceV1 {
  data class Config(
    val rejectedTimestampWithinDuration: Duration,
  )

  private val txRejectionCounter = metricsFacade.createCounter(
    category = LineaMetricsCategory.TX_EXCLUSION_API,
    name = "transactions.rejected",
    description = "Counter of rejected transactions reported to Transaction Exclusion API service",
  )

  override fun saveRejectedTransaction(
    rejectedTransaction: RejectedTransaction,
  ): SafeFuture<
    Result<SaveRejectedTransactionStatus, TransactionExclusionError>,
    > {
    return this.repository.saveNewRejectedTransaction(rejectedTransaction)
      .handleComposed { _, error ->
        if (error != null) {
          if (error is DuplicatedRecordException) {
            SafeFuture.completedFuture(
              Ok(SaveRejectedTransactionStatus.DUPLICATE_ALREADY_SAVED_BEFORE),
            )
          } else {
            SafeFuture.completedFuture(
              Err(TransactionExclusionError(ErrorType.SERVER_ERROR, error.message ?: "")),
            )
          }
        } else {
          txRejectionCounter.increment()
          SafeFuture.completedFuture(Ok(SaveRejectedTransactionStatus.SAVED))
        }
      }
  }

  override fun getTransactionExclusionStatus(
    txHash: ByteArray,
  ): SafeFuture<Result<RejectedTransaction?, TransactionExclusionError>> {
    return this.repository.findRejectedTransactionByTxHash(
      txHash = txHash,
      notRejectedBefore = clock.now().minus(config.rejectedTimestampWithinDuration),
    )
      .handleComposed { result, error ->
        if (error != null) {
          SafeFuture.completedFuture(
            Err(TransactionExclusionError(ErrorType.SERVER_ERROR, error.message ?: "")),
          )
        } else {
          SafeFuture.completedFuture(Ok(result))
        }
      }
  }
}
