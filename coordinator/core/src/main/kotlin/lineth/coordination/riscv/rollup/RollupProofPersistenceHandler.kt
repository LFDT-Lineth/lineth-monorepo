package lineth.coordination.riscv.rollup

import linea.clients.RollupProofResponseV1
import linea.domain.RiscvRollupRecord
import lineth.persistence.RiscvRollupsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture

class RollupProofPersistenceHandler(
  private val rollupsDao: RiscvRollupsDao,
) : RollupProofHandler {
  override fun acceptNewRollupProof(
    proof: RollupProofResponseV1,
    context: RollupProofPoller.ProofContext,
  ): SafeFuture<*> {
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
