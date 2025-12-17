package net.consensys.zkevm.persistence.dao.batch.persistence

import net.consensys.zkevm.domain.Batch
import tech.pegasys.teku.infrastructure.async.SafeFuture

interface BatchesDao {
  fun saveNewBatch(batch: Batch): SafeFuture<Unit>

  fun findHighestConsecutiveEndBlockNumberFromBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Long?>

  fun deleteBatchesUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int>

  fun deleteBatchesAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int>
}
