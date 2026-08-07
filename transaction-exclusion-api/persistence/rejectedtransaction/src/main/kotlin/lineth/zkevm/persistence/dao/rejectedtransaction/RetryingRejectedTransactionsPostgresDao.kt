package lineth.zkevm.persistence.dao.rejectedtransaction

import lineth.persistence.db.PersistenceRetryer
import lineth.transactionexclusion.RejectedTransaction
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
