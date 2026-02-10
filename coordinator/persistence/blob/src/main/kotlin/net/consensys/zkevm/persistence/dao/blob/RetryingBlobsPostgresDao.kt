package net.consensys.zkevm.persistence.dao.blob

import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

class RetryingBlobsPostgresDao(
  private val delegate: BlobsPostgresDao,
  private val persistenceRetryer: PersistenceRetryer,
) : BlobsDao {
  override fun saveNewBlob(blobRecord: BlobRecord): SafeFuture<Unit> {
    return persistenceRetryer.retryQuery({ delegate.saveNewBlob(blobRecord) })
  }

  override fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<BlobRecord>> {
    return persistenceRetryer.retryQuery({
      delegate.getConsecutiveBlobsFromBlockNumber(
        startingBlockNumberInclusive,
        endBlockCreatedBefore,
      )
    })
  }

  override fun findBlobByStartBlockNumber(startBlockNumber: ULong): SafeFuture<BlobRecord?> {
    return persistenceRetryer.retryQuery({ delegate.findBlobByStartBlockNumber(startBlockNumber) })
  }

  override fun findBlobByEndBlockNumber(endBlockNumber: ULong): SafeFuture<BlobRecord?> {
    return persistenceRetryer.retryQuery({ delegate.findBlobByEndBlockNumber(endBlockNumber) })
  }

  override fun deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive) })
  }

  override fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteBlobsAfterBlockNumber(startingBlockNumberInclusive) })
  }
}
