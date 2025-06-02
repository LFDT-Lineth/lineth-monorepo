package net.consensys.zkevm.persistence.dao.batch.persistence

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import net.consensys.zkevm.domain.createBatch
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
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class RetryingBatchesPostgresDaoTest {
  private lateinit var retryingBatchesPostgresDao: RetryingBatchesPostgresDao
  private val delegateBatchesDao = mock<BatchesPostgresDao>()
  private val batch = createBatch(
    startBlockNumber = 0L,
    endBlockNumber = 10L,
  )

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    retryingBatchesPostgresDao = RetryingBatchesPostgresDao(
      delegate = delegateBatchesDao,
      PersistenceRetryer(
        vertx = vertx,
        PersistenceRetryer.Config(
          backoffDelay = 1.milliseconds,
        ),
      ),
    )

    whenever(delegateBatchesDao.saveNewBatch(eq(batch)))
      .thenReturn(SafeFuture.completedFuture(Unit))

    whenever(delegateBatchesDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(eq(0L)))
      .thenReturn(SafeFuture.completedFuture(10L))

    whenever(delegateBatchesDao.deleteBatchesUpToEndBlockNumber(eq(10L)))
      .thenReturn(SafeFuture.completedFuture(1))

    whenever(delegateBatchesDao.deleteBatchesAfterBlockNumber(eq(0L)))
      .thenReturn(SafeFuture.completedFuture(1))
  }

  @Test
  fun `retrying batches dao should delegate all queries to standard dao`() {
    retryingBatchesPostgresDao.saveNewBatch(
      batch,
    )
    verify(delegateBatchesDao, times(1)).saveNewBatch(batch)

    retryingBatchesPostgresDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(0L)
    verify(delegateBatchesDao, times(1))
      .findHighestConsecutiveEndBlockNumberFromBlockNumber(eq(0L))

    retryingBatchesPostgresDao.deleteBatchesUpToEndBlockNumber(10L)
    verify(delegateBatchesDao, times(1))
      .deleteBatchesUpToEndBlockNumber(eq(10L))

    retryingBatchesPostgresDao.deleteBatchesAfterBlockNumber(0L)
    verify(delegateBatchesDao, times(1))
      .deleteBatchesAfterBlockNumber(eq(0L))
  }
}
