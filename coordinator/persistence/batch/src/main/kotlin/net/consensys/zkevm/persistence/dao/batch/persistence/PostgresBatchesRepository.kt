package net.consensys.zkevm.persistence.dao.batch.persistence

import net.consensys.zkevm.domain.Batch
import net.consensys.zkevm.persistence.BatchesRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture

class PostgresBatchesRepository(
  private val batchesDao: BatchesDao
) : BatchesRepository {
  override fun saveNewBatch(batch: Batch): SafeFuture<Unit> {
    return batchesDao.saveNewBatch(batch)
  }

  override fun findHighestConsecutiveEndBlockNumberFromBlockNumber(
    startingBlockNumberInclusive: Long
  ): SafeFuture<Long?> {
    return batchesDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(startingBlockNumberInclusive)
  }

  override fun deleteBatchesUpToEndBlockNumber(
    endBlockNumberInclusive: Long
  ): SafeFuture<Int> {
    return batchesDao.deleteBatchesUpToEndBlockNumber(endBlockNumberInclusive)
  }

  override fun deleteBatchesAfterBlockNumber(
    startingBlockNumberInclusive: Long
  ): SafeFuture<Int> {
    return batchesDao.deleteBatchesAfterBlockNumber(startingBlockNumberInclusive)
  }
}
