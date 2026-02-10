package net.consensys.zkevm.persistence.dao.rejectedtransaction

import net.consensys.linea.transactionexclusion.RejectedTransaction
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

class RetryingRejectedTransactionsPostgresDao(
  private val delegate: RejectedTransactionsPostgresDao,
  private val persistenceRetryer: PersistenceRetryer,
) : RejectedTransactionsDao {
  override fun saveNewRejectedTransaction(rejectedTransaction: RejectedTransaction): SafeFuture<Unit> {
    return persistenceRetryer.retryQuery({ delegate.saveNewRejectedTransaction(rejectedTransaction) })
  }

  override fun findRejectedTransactionByTxHash(
    txHash: ByteArray,
    notRejectedBefore: Instant,
  ): SafeFuture<RejectedTransaction?> {
    return persistenceRetryer.retryQuery({ delegate.findRejectedTransactionByTxHash(txHash, notRejectedBefore) })
  }

  override fun deleteRejectedTransactions(createdBefore: Instant): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteRejectedTransactions(createdBefore) })
  }
}
