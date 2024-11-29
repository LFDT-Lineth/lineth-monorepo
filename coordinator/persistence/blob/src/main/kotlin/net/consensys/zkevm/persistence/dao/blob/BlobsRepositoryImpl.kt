package net.consensys.zkevm.persistence.dao.blob

import kotlinx.datetime.Instant
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.persistence.BlobsRepository
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import tech.pegasys.teku.infrastructure.async.SafeFuture

class BlobsRepositoryImpl(
  private val blobsDao: BlobsDao
) : BlobsRepository {
  override fun saveNewBlob(blobRecord: BlobRecord): SafeFuture<Unit> {
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
    endBlockCreatedBefore: Instant
  ): SafeFuture<List<BlobRecord>> {
    return blobsDao.getConsecutiveBlobsFromBlockNumber(
      startingBlockNumberInclusive.toULong(),
      endBlockCreatedBefore
    )
  }

  override fun findBlobByStartBlockNumber(startBlockNumber: Long): SafeFuture<BlobRecord?> {
    return blobsDao.findBlobByStartBlockNumber(startBlockNumber.toULong())
  }

  override fun findBlobByEndBlockNumber(endBlockNumber: Long): SafeFuture<BlobRecord?> {
    return blobsDao.findBlobByEndBlockNumber(endBlockNumber.toULong())
  }

  override fun deleteBlobsUpToEndBlockNumber(
    endBlockNumberInclusive: ULong
  ): SafeFuture<Int> {
    return blobsDao.deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive)
  }

  override fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> {
    return blobsDao.deleteBlobsAfterBlockNumber(startingBlockNumberInclusive)
  }
}
