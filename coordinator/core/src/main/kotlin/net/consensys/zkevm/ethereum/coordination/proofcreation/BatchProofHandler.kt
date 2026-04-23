package net.consensys.zkevm.ethereum.coordination.proofcreation

import linea.domain.Batch
import linea.domain.ExecutionProofIndex
import linea.error.DuplicatedRecordException
import linea.persistence.BatchesRepository
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface BatchProofHandler {
  fun acceptNewBatch(batch: Batch): SafeFuture<*>
}

fun interface BatchProofRequestHandler {
  fun acceptNewBatchProofRequest(proofIndex: ExecutionProofIndex, unProvenBatch: Batch)
}

class BatchProofHandlerImpl(
  private val batchesRepository: BatchesRepository,
) : BatchProofHandler {
  private val log = LogManager.getLogger(this::class.java)
  override fun acceptNewBatch(batch: Batch): SafeFuture<Unit> {
    return batchesRepository.saveNewBatch(batch)
      .exceptionallyCompose { th ->
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
