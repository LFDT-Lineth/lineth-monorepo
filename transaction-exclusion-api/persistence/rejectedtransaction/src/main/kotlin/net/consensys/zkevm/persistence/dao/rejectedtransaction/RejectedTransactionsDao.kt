package net.consensys.zkevm.persistence.dao.rejectedtransaction

import net.consensys.linea.transactionexclusion.RejectedTransaction
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

interface RejectedTransactionsDao {
  fun saveNewRejectedTransaction(rejectedTransaction: RejectedTransaction): SafeFuture<Unit>

  fun findRejectedTransactionByTxHash(
    txHash: ByteArray,
    notRejectedBefore: Instant = Instant.DISTANT_PAST,
  ): SafeFuture<RejectedTransaction?>

  fun deleteRejectedTransactions(createdBefore: Instant): SafeFuture<Int>
}
