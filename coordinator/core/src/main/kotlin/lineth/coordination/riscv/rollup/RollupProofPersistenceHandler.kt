package lineth.coordination.riscv.rollup

import linea.clients.RollupProofResponseV1
import linea.domain.RiscvRollupRecord
import linea.kotlin.encodeHex
import lineth.persistence.RiscvRollupsDao
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Persists each rollup proof as a [RiscvRollupRecord] with its blobs. A proof whose DA outputs differ from what the coordinator built
 * is not persisted: the record would not match its blobs. The returned future then fails, so the proof stays in progress.
 */
class RollupProofPersistenceHandler(
  private val rollupsDao: RiscvRollupsDao,
  private val log: Logger = LogManager.getLogger(RollupProofPersistenceHandler::class.java),
) : RollupProofHandler {
  override fun acceptNewRollupProof(
    proof: RollupProofResponseV1,
    context: RollupProofPoller.ProofContext,
  ): SafeFuture<*> {
    val inputs = proof.publicInputs
    if (!inputs.parentDataRollingHash.contentEquals(context.parentDataRollingHash) ||
      !inputs.endDataRollingHash.contentEquals(context.dataRollingHash) ||
      inputs.parentSharedBlobCarryOverTailBytes.toLong() != context.parentSharedBlobCarryOverTailBytes.toLong() ||
      inputs.finalSharedBlobCarryOverTailBytes.toLong() != context.finalSharedBlobCarryOverTailBytes.toLong()
    ) {
      val message = "rollup proof DA outputs differ from the request: rollup=${context.proofIndex.intervalString()} " +
        "parentDataRollingHash=${inputs.parentDataRollingHash.encodeHex()} " +
        "expected=${context.parentDataRollingHash.encodeHex()} " +
        "endDataRollingHash=${inputs.endDataRollingHash.encodeHex()} expected=${context.dataRollingHash.encodeHex()} " +
        "parentSharedBlobCarryOverTailBytes=${inputs.parentSharedBlobCarryOverTailBytes} " +
        "expected=${context.parentSharedBlobCarryOverTailBytes} " +
        "finalSharedBlobCarryOverTailBytes=${inputs.finalSharedBlobCarryOverTailBytes} " +
        "expected=${context.finalSharedBlobCarryOverTailBytes}"
      log.error(message)
      return SafeFuture.failedFuture<Unit>(IllegalStateException(message))
    }
    val record = RiscvRollupRecord(
      startBlockNumber = context.proofIndex.startBlockNumber,
      endBlockNumber = context.proofIndex.endBlockNumber,
      startBlockTimestamp = context.startBlockTimestamp,
      endBlockTimestamp = context.endBlockTimestamp,
      parentDataRollingHash = context.parentDataRollingHash,
      dataRollingHash = context.dataRollingHash,
      totalBatchesCount = context.totalBatchesCount.toUInt(),
      blobsData = context.blobsData,
      proofHash = context.proofIndex.hash,
      parentSharedBlobCarryOverTailBytes = context.parentSharedBlobCarryOverTailBytes,
      parentBlobDataRollingHash = context.parentBlobDataRollingHash,
      finalSharedBlobCarryOverTailBytes = context.finalSharedBlobCarryOverTailBytes,
    )
    return rollupsDao.saveRollupWithBlobs(record)
  }
}
