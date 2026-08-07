package lineth.persistence

import lineth.domain.BlobRecord
import lineth.domain.BlobRecordV2
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

interface BlobsRepositoryG<T> {
  fun saveNewBlob(blobRecord: T): SafeFuture<Unit>

  fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: Long,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<T>>

  fun findBlobByStartBlockNumber(startBlockNumber: Long): SafeFuture<T?>

  fun findBlobByEndBlockNumber(endBlockNumber: Long): SafeFuture<T?>

  fun deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int>

  fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int>
}

interface BlobsRepository : BlobsRepositoryG<BlobRecord>
interface BlobsRepositoryV2 : BlobsRepositoryG<BlobRecordV2>
