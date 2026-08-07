package lineth.persistence.conflation

import lineth.domain.BlobRecord
import lineth.domain.BlobRecordV2
import lineth.persistence.db.PersistenceRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

abstract class RetryingBlobsPostgresDaoG<T>(
  private val delegate: BlobsDaoG<T>,
  private val persistenceRetryer: PersistenceRetryer,
) : BlobsDaoG<T> {
  override fun saveNewBlob(blobRecord: T): SafeFuture<Unit> {
    return persistenceRetryer.retryQuery({ delegate.saveNewBlob(blobRecord) })
  }

  override fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<T>> {
    return persistenceRetryer.retryQuery({
      delegate.getConsecutiveBlobsFromBlockNumber(
        startingBlockNumberInclusive,
        endBlockCreatedBefore,
      )
    })
  }

  override fun findBlobByStartBlockNumber(startBlockNumber: ULong): SafeFuture<T?> {
    return persistenceRetryer.retryQuery({ delegate.findBlobByStartBlockNumber(startBlockNumber) })
  }

  override fun findBlobByEndBlockNumber(endBlockNumber: ULong): SafeFuture<T?> {
    return persistenceRetryer.retryQuery({ delegate.findBlobByEndBlockNumber(endBlockNumber) })
  }

  override fun deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive) })
  }

  override fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteBlobsAfterBlockNumber(startingBlockNumberInclusive) })
  }
}

class RetryingBlobsPostgresDao(
  private val delegate: BlobsDao,
  private val persistenceRetryer: PersistenceRetryer,
) : RetryingBlobsPostgresDaoG<BlobRecord>(delegate, persistenceRetryer), BlobsDao

class RetryingBlobsPostgresDaoV2(
  private val delegate: BlobsDaoV2,
  private val persistenceRetryer: PersistenceRetryer,
) : RetryingBlobsPostgresDaoG<BlobRecordV2>(delegate, persistenceRetryer), BlobsDaoV2
