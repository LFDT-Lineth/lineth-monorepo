package linea.coordinator.app.conflationbacktesting

import linea.domain.BlobRecord
import linea.persistence.BlobsRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentHashMap
import kotlin.time.Instant

/**
 * In-memory [BlobsRepository] for conflation backtesting, which has no Postgres-backed blobs
 * table. Fed by [saveNewBlob] as blobs are proven.
 */
class InMemoryBlobsRepository : BlobsRepository {
  private val blobsByStartBlockNumber = ConcurrentHashMap<Long, BlobRecord>()
  private val blobsByEndBlockNumber = ConcurrentHashMap<Long, BlobRecord>()

  override fun saveNewBlob(blobRecord: BlobRecord): SafeFuture<Unit> {
    blobsByStartBlockNumber[blobRecord.startBlockNumber.toLong()] = blobRecord
    blobsByEndBlockNumber[blobRecord.endBlockNumber.toLong()] = blobRecord
    return SafeFuture.completedFuture(Unit)
  }

  override fun getConsecutiveBlobsFromBlockNumber(
    startingBlockNumberInclusive: Long,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<BlobRecord>> {
    val result = mutableListOf<BlobRecord>()
    var expectedNext = startingBlockNumberInclusive
    while (true) {
      val blob = blobsByStartBlockNumber[expectedNext] ?: break
      if (blob.endBlockTime >= endBlockCreatedBefore) break
      result.add(blob)
      expectedNext = blob.endBlockNumber.toLong() + 1
    }
    return SafeFuture.completedFuture(result)
  }

  override fun findBlobByStartBlockNumber(startBlockNumber: Long): SafeFuture<BlobRecord?> {
    return SafeFuture.completedFuture(blobsByStartBlockNumber[startBlockNumber])
  }

  override fun findBlobByEndBlockNumber(endBlockNumber: Long): SafeFuture<BlobRecord?> {
    return SafeFuture.completedFuture(blobsByEndBlockNumber[endBlockNumber])
  }

  override fun deleteBlobsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> {
    val toDelete = blobsByEndBlockNumber.keys.filter { it <= endBlockNumberInclusive.toLong() }
    toDelete.forEach { endBlockNumber ->
      blobsByEndBlockNumber.remove(endBlockNumber)?.let { blobsByStartBlockNumber.remove(it.startBlockNumber.toLong()) }
    }
    return SafeFuture.completedFuture(toDelete.size)
  }

  override fun deleteBlobsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> {
    val toDelete = blobsByStartBlockNumber.keys.filter { it >= startingBlockNumberInclusive.toLong() }
    toDelete.forEach { startBlockNumber ->
      blobsByStartBlockNumber.remove(startBlockNumber)?.let { blobsByEndBlockNumber.remove(it.endBlockNumber.toLong()) }
    }
    return SafeFuture.completedFuture(toDelete.size)
  }
}
