package net.consensys.zkevm.persistence

import kotlinx.datetime.Instant
import net.consensys.zkevm.domain.BlobRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface BlobsRepository {
  fun saveNewBlob(blobRecord: BlobRecord): SafeFuture<Unit>

  fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: Long,
    endBlockCreatedBefore: Instant
  ): SafeFuture<List<BlobRecord>>

  fun findBlobByStartBlockNumber(
    startBlockNumber: Long
  ): SafeFuture<BlobRecord?>

  fun findBlobByEndBlockNumber(
    endBlockNumber: Long
  ): SafeFuture<BlobRecord?>

  fun deleteBlobsUpToEndBlockNumber(
    endBlockNumberInclusive: ULong
  ): SafeFuture<Int>

  fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int>
}
