package net.consensys.zkevm.persistence.dao.batch.persistence

import net.consensys.zkevm.domain.Batch
import net.consensys.zkevm.ethereum.coordination.proofcreation.BatchProofHandler
import net.consensys.zkevm.persistence.BatchesRepository
import net.consensys.zkevm.persistence.db.DuplicatedRecordException
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture

class BatchProofHandlerImpl(
  private val batchesRepository: BatchesRepository,
) : BatchProofHandler {
  private val log = LogManager.getLogger(this::class.java)
  override fun acceptNewBatch(batch: Batch): SafeFuture<Unit> {
    return batchesRepository.saveNewBatch(batch)
      .exceptionally { th ->
        if (th is DuplicatedRecordException) {
          log.debug(
            "Ignoring Batch already persisted error. batch={} errorMessage={}",
            batch.intervalString(),
            th.message,
          )
          SafeFuture.completedFuture(Unit)
        } else {
          SafeFuture.failedFuture(th)
        }
      }
  }
}
