package lineth.coordination.riscv.rollup

import linea.clients.RollupProofResponseV1
import linea.domain.BlobRecordV2
import lineth.persistence.BlobsRepositoryV2
import tech.pegasys.teku.infrastructure.async.SafeFuture

class RollupProofPersistenceHandler(
  private val blobsRepository: BlobsRepositoryV2,
) : RollupProofHandler {
  override fun acceptNewRollupProof(
    proof: RollupProofResponseV1,
    context: RollupProofPoller.ProofContext,
  ): SafeFuture<*> {
    val record = BlobRecordV2(
      startBlockNumber = context.proofIndex.startBlockNumber,
      endBlockNumber = context.proofIndex.endBlockNumber,
      startBlockTimestamp = context.startBlockTimestamp,
      endBlockTimestamp = context.endBlockTimestamp,
      parentDataRollingHash = context.parentDataRollingHash,
      dataRollingHash = context.dataRollingHash,
      totalConflationsCount = context.totalConflationsCount.toUInt(),
      blobsData = context.blobsData,
      proofHash = context.proofIndex.hash,
      endOffset = context.endOffset,
    )
    return blobsRepository.saveNewBlob(record)
  }
}
