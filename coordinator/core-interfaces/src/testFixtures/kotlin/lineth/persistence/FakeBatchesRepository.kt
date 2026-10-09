package lineth.persistence

import linea.domain.Batch
import linea.error.DuplicatedRecordException
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentSkipListMap

/**
 * In-memory [BatchesRepository] mirroring the semantics of `BatchesPostgresDao`.
 * Batches are keyed by start block number; when several versions share a start block,
 * the one with the lowest end block number wins, as in the Postgres query.
 */
class FakeBatchesRepository(
  val batches: ConcurrentSkipListMap<ULong, Batch> = ConcurrentSkipListMap(),
) : BatchesRepository {

  @Synchronized
  override fun saveNewBatch(batch: Batch): SafeFuture<Unit> {
    val existing = batches[batch.startBlockNumber]
    if (existing != null) {
      if (existing.endBlockNumber == batch.endBlockNumber) {
        return SafeFuture.failedFuture(
          DuplicatedRecordException(
            "Batch startBlockNumber=${batch.startBlockNumber}, endBlockNumber=${batch.endBlockNumber} " +
              "is already persisted!",
          ),
        )
      }
      if (existing.endBlockNumber < batch.endBlockNumber) return SafeFuture.completedFuture(Unit)
    }
    batches[batch.startBlockNumber] = batch
    return SafeFuture.completedFuture(Unit)
  }

  override fun findHighestConsecutiveEndBlockNumberFromBlockNumber(
    startingBlockNumberInclusive: Long,
  ): SafeFuture<Long?> {
    var current = batches[startingBlockNumberInclusive.toULong()]
      ?: return SafeFuture.completedFuture(null)
    while (true) {
      current = batches[current.endBlockNumber + 1UL] ?: break
    }
    return SafeFuture.completedFuture(current.endBlockNumber.toLong())
  }

  override fun findBatchesByBlockRange(startBlockNumber: Long, endBlockNumber: Long): SafeFuture<List<Batch>> {
    return SafeFuture.completedFuture(
      batches.values.filter {
        it.startBlockNumber.toLong() >= startBlockNumber && it.endBlockNumber.toLong() <= endBlockNumber
      },
    )
  }

  @Synchronized
  override fun deleteBatchesUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int> {
    return SafeFuture.completedFuture(removeIf { it.endBlockNumber.toLong() <= endBlockNumberInclusive })
  }

  @Synchronized
  override fun deleteBatchesAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int> {
    return SafeFuture.completedFuture(removeIf { it.startBlockNumber.toLong() >= startingBlockNumberInclusive })
  }

  private fun removeIf(predicate: (Batch) -> Boolean): Int {
    val toRemove = batches.values.filter(predicate)
    toRemove.forEach { batches.remove(it.startBlockNumber) }
    return toRemove.size
  }
}
