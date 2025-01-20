package net.consensys.linea.transactionexclusion.service

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import kotlinx.datetime.Clock
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.transactionexclusion.ErrorType
import net.consensys.linea.transactionexclusion.RejectedTransaction
import net.consensys.linea.transactionexclusion.TransactionExclusionError
import net.consensys.linea.transactionexclusion.TransactionExclusionServiceV1
import net.consensys.linea.transactionexclusion.TransactionExclusionServiceV1.SaveRejectedTransactionStatus
import net.consensys.zkevm.persistence.dao.rejectedtransaction.RejectedTransactionsDao
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class TransactionExclusionServiceV1Impl(
  private val config: Config,
  private val repository: RejectedTransactionsDao,
  metricsFacade: MetricsFacade,
  private val clock: Clock = Clock.System
) : TransactionExclusionServiceV1 {
  data class Config(
    val rejectedTimestampWithinDuration: Duration
  )

  private val txRejectionCounter = metricsFacade.createCounter(
    category = LineaMetricsCategory.TX_EXCLUSION_API,
    name = "transactions.rejected",
    description = "Counter of rejected transactions reported to Transaction Exclusion API service"
  )

  override fun saveRejectedTransaction(
    rejectedTransaction: RejectedTransaction
  ): SafeFuture<
    Result<SaveRejectedTransactionStatus, TransactionExclusionError>
    > {
    return this.repository.saveNewRejectedTransaction(rejectedTransaction)
      .handleComposed { _, error ->
        if (error != null) {
          if (error is DuplicatedRecordException) {
            SafeFuture.completedFuture(
              Ok(SaveRejectedTransactionStatus.DUPLICATE_ALREADY_SAVED_BEFORE)
            )
          } else {
            SafeFuture.completedFuture(
              Err(TransactionExclusionError(ErrorType.SERVER_ERROR, error.message ?: ""))
            )
          }
        } else {
          txRejectionCounter.increment()
          SafeFuture.completedFuture(Ok(SaveRejectedTransactionStatus.SAVED))
        }
      }
  }

  override fun getTransactionExclusionStatus(
    txHash: ByteArray
  ): SafeFuture<Result<RejectedTransaction?, TransactionExclusionError>> {
    return this.repository.findRejectedTransactionByTxHash(
      txHash = txHash,
      notRejectedBefore = clock.now().minus(config.rejectedTimestampWithinDuration)
    )
      .handleComposed { result, error ->
        if (error != null) {
          SafeFuture.completedFuture(
            Err(TransactionExclusionError(ErrorType.SERVER_ERROR, error.message ?: ""))
          )
        } else {
          SafeFuture.completedFuture(Ok(result))
        }
      }
  }
}
