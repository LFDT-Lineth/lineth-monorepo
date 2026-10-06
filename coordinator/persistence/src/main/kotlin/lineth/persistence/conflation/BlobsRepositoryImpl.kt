package lineth.persistence.conflation

import linea.domain.BlobRecord
import linea.domain.BlobRecordV2
import linea.error.DuplicatedRecordException
import lineth.persistence.BlobsRepository
import lineth.persistence.BlobsRepositoryG
import lineth.persistence.BlobsRepositoryV2
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

abstract class BlobsRepositoryImplG<T>(
  private val blobsDao: BlobsDaoG<T>,
) : BlobsRepositoryG<T> {
  override fun saveNewBlob(blobRecord: T): SafeFuture<Unit> {
    return blobsDao.saveNewBlob(blobRecord)
      .exceptionallyCompose { error ->
        if (error is DuplicatedRecordException) {
          SafeFuture.completedFuture(Unit)
        } else {
          SafeFuture.failedFuture(error)
        }
      }
  }

  override fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: Long,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<T>> {
    return blobsDao.getConsecutiveBlobsFromBlockNumber(
      startingBlockNumberInclusive.toULong(),
      endBlockCreatedBefore,
    )
  }

  override fun findBlobByStartBlockNumber(startBlockNumber: Long): SafeFuture<T?> {
    return blobsDao.findBlobByStartBlockNumber(startBlockNumber.toULong())
  }

  override fun findBlobByEndBlockNumber(endBlockNumber: Long): SafeFuture<T?> {
    return blobsDao.findBlobByEndBlockNumber(endBlockNumber.toULong())
  }

  override fun deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> {
    return blobsDao.deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive)
  }

  override fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> {
    return blobsDao.deleteBlobsAfterBlockNumber(startingBlockNumberInclusive)
  }
}

class BlobsRepositoryImpl(
  private val blobsDao: BlobsDao,
) : BlobsRepositoryImplG<BlobRecord>(blobsDao), BlobsRepository

class BlobsRepositoryImplV2(
  private val blobsDao: BlobsDaoV2,
) : BlobsRepositoryImplG<BlobRecordV2>(blobsDao), BlobsRepositoryV2
