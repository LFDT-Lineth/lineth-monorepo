package net.consensys.zkevm.persistence.dao.rejectedtransaction

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import net.consensys.linea.transactionexclusion.test.defaultRejectedTransaction
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito.mock
import org.mockito.Mockito.times
import org.mockito.kotlin.eq
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Clock
import kotlin.time.Duration.Companion.days
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class RetryingRejectedTransactionsPostgresDaoTest {
  private lateinit var retryingRejectedTransactionsPostgresDao: RetryingRejectedTransactionsPostgresDao
  private val delegateRejectedTransactionsDao = mock<RejectedTransactionsPostgresDao>()
  private val now = Clock.System.now()
  private val notRejectedBefore = now.minus(24.hours)
  private val createdBefore = now.minus(7.days)
  private val rejectedTransaction = defaultRejectedTransaction

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    retryingRejectedTransactionsPostgresDao = RetryingRejectedTransactionsPostgresDao(
      delegate = delegateRejectedTransactionsDao,
      PersistenceRetryer(
        vertx = vertx,
        PersistenceRetryer.Config(
          backoffDelay = 1.milliseconds,
        ),
      ),
    )

    whenever(delegateRejectedTransactionsDao.saveNewRejectedTransaction(eq(rejectedTransaction)))
      .thenReturn(SafeFuture.completedFuture(Unit))

    whenever(
      delegateRejectedTransactionsDao.findRejectedTransactionByTxHash(
        eq(rejectedTransaction.transactionInfo.hash),
        eq(notRejectedBefore),
      ),
    )
      .thenReturn(SafeFuture.completedFuture(null))

    whenever(delegateRejectedTransactionsDao.deleteRejectedTransactions(eq(createdBefore)))
      .thenReturn(SafeFuture.completedFuture(0))
  }

  @Test
  fun `retrying rejected transactions dao should delegate all queries to standard dao`() {
    retryingRejectedTransactionsPostgresDao.saveNewRejectedTransaction(rejectedTransaction)
    verify(delegateRejectedTransactionsDao, times(1)).saveNewRejectedTransaction(eq(rejectedTransaction))

    retryingRejectedTransactionsPostgresDao.findRejectedTransactionByTxHash(
      rejectedTransaction.transactionInfo.hash,
      notRejectedBefore,
    )
    verify(delegateRejectedTransactionsDao, times(1)).findRejectedTransactionByTxHash(
      eq(rejectedTransaction.transactionInfo.hash),
      eq(notRejectedBefore),
    )

    retryingRejectedTransactionsPostgresDao.deleteRejectedTransactions(createdBefore)
    verify(delegateRejectedTransactionsDao, times(1)).deleteRejectedTransactions(
      eq(createdBefore),
    )
  }
}
