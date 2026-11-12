package lineth.persistence

import linea.domain.BlobRecord
import linea.domain.BlobRecordV2
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
interface BlobsRepositoryV2 : BlobsRepositoryG<BlobRecordV2> {
  /**
   * End block of the last rollup row in the gap-free run starting at [startingBlockNumberInclusive], or null if no row
   * starts there. Reads block numbers only.
   */
  fun findHighestConsecutiveEndBlockNumberFromBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<ULong?>
}
