package linea.persistence.conflation

import linea.domain.BlobRecord
import linea.domain.BlobRecordV2
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

interface BlobsDaoG<T> {
  fun saveNewBlob(blobRecord: T): SafeFuture<Unit>

  fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<T>>

  fun findBlobByStartBlockNumber(startBlockNumber: ULong): SafeFuture<T?>

  fun findBlobByEndBlockNumber(endBlockNumber: ULong): SafeFuture<T?>

  fun deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int>

  fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int>
}

interface BlobsDao : BlobsDaoG<BlobRecord>
interface BlobsDaoV2 : BlobsDaoG<BlobRecordV2>

