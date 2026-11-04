package lineth.coordination.riscv.rollup

import linea.clients.RollupProofResponseV1
import linea.domain.BlobRecordV2
import linea.kotlin.encodeHex
import lineth.persistence.BlobsRepositoryV2
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Persists each rollup proof as a [BlobRecordV2] row. A proof whose DA outputs differ from what the coordinator built
 * is not persisted: the row would not match its blobs. The returned future then fails, so the proof stays in progress.
 */
class RollupProofPersistenceHandler(
  private val blobsRepository: BlobsRepositoryV2,
  private val log: Logger = LogManager.getLogger(RollupProofPersistenceHandler::class.java),
) : RollupProofHandler {
  override fun acceptNewRollupProof(
    proof: RollupProofResponseV1,
    context: RollupProofPoller.ProofContext,
  ): SafeFuture<*> {
    val inputs = proof.publicInputs
    if (!inputs.parentDataRollingHash.contentEquals(context.parentDataRollingHash) ||
      !inputs.endDataRollingHash.contentEquals(context.dataRollingHash) ||
      inputs.parentDataTailTakeBytes != context.parentDataTailTakeBytes ||
      inputs.finalDataTailDiscardBytes != context.finalDataTailDiscardBytes
    ) {
      val message = "rollup proof DA outputs differ from the request: rollup=${context.proofIndex.intervalString()} " +
        "parentDataRollingHash=${inputs.parentDataRollingHash.encodeHex()} " +
        "expected=${context.parentDataRollingHash.encodeHex()} " +
        "endDataRollingHash=${inputs.endDataRollingHash.encodeHex()} expected=${context.dataRollingHash.encodeHex()} " +
        "parentDataTailTakeBytes=${inputs.parentDataTailTakeBytes} expected=${context.parentDataTailTakeBytes} " +
        "finalDataTailDiscardBytes=${inputs.finalDataTailDiscardBytes} expected=${context.finalDataTailDiscardBytes}"
      log.error(message)
      return SafeFuture.failedFuture<Unit>(IllegalStateException(message))
    }
    val record = BlobRecordV2(
      startBlockNumber = context.proofIndex.startBlockNumber,
      endBlockNumber = context.proofIndex.endBlockNumber,
      startBlockTimestamp = context.startBlockTimestamp,
      endBlockTimestamp = context.endBlockTimestamp,
      parentDataRollingHash = context.parentDataRollingHash,
      dataRollingHash = context.dataRollingHash,
      totalBatchesCount = context.totalBatchesCount.toUInt(),
      blobsData = context.blobsData,
      proofHash = context.proofIndex.hash,
      parentDataTailTakeBytes = context.parentDataTailTakeBytes,
      boundaryPrevDataRollingHash = context.boundaryPrevDataRollingHash,
      finalDataTailDiscardBytes = context.finalDataTailDiscardBytes,
    )
    return blobsRepository.saveNewBlob(record)
  }
}
