package net.consensys.linea.transactionexclusion

import com.github.michaelbull.result.Result
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface TransactionExclusionServiceV1 {
  enum class SaveRejectedTransactionStatus {
    SAVED,
    DUPLICATE_ALREADY_SAVED_BEFORE,
  }

  fun saveRejectedTransaction(
    rejectedTransaction: RejectedTransaction,
  ): SafeFuture<Result<SaveRejectedTransactionStatus, TransactionExclusionError>>

  fun getTransactionExclusionStatus(
    txHash: ByteArray,
  ): SafeFuture<Result<RejectedTransaction?, TransactionExclusionError>>
}
