package linea.persistence.conflation

import linea.domain.Batch
import linea.persistence.db.PersistenceRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture

class RetryingBatchesPostgresDao(
  private val delegate: BatchesPostgresDao,
  private val persistenceRetryer: PersistenceRetryer,
) : BatchesDao {
  override fun saveNewBatch(batch: Batch): SafeFuture<Unit> {
    return persistenceRetryer.retryQuery({ delegate.saveNewBatch(batch) })
  }

  override fun findHighestConsecutiveEndBlockNumberFromBlockNumber(
    startingBlockNumberInclusive: Long,
  ): SafeFuture<Long?> {
    return persistenceRetryer.retryQuery(
      { delegate.findHighestConsecutiveEndBlockNumberFromBlockNumber(startingBlockNumberInclusive) },
    )
  }

  override fun deleteBatchesUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteBatchesUpToEndBlockNumber(endBlockNumberInclusive) })
  }

  override fun deleteBatchesAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteBatchesAfterBlockNumber(startingBlockNumberInclusive) })
  }
}
