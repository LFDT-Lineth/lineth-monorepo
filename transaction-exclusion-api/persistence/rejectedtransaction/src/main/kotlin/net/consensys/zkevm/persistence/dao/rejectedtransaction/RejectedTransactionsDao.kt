package net.consensys.zkevm.persistence.dao.rejectedtransaction

import kotlinx.datetime.Instant
import net.consensys.linea.transactionexclusion.RejectedTransaction
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface RejectedTransactionsDao {
  fun saveNewRejectedTransaction(rejectedTransaction: RejectedTransaction): SafeFuture<Unit>

  fun findRejectedTransactionByTxHash(
    txHash: ByteArray,
    notRejectedBefore: Instant = Instant.DISTANT_PAST,
  ): SafeFuture<RejectedTransaction?>

  fun deleteRejectedTransactions(createdBefore: Instant): SafeFuture<Int>
}
