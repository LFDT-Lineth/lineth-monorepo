package lineth.coordination.riscv.rollup

import linea.da.DaStreamPosition
import linea.da.RollupResumePoint
import linea.da.RollupResumePointProvider
import linea.domain.RiscvGenesisDataRollingHashProvider
import linea.domain.RiscvRollupRecord
import lineth.persistence.RiscvRollupsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

/**
 * Resumes after the last persisted rollup row that follows the L1 finalized block F without gaps.
 *
 * - F before the RISC-V cutover: no RISC-V rollup is finalized yet, so the anchor is the genesis position at F. This
 *   assumes the pre-RISC-V flow has finalized every pre-RISC-V block, so F is the last pre-RISC-V block.
 * - F at or after the cutover: the anchor is the row ending at F. A missing row is an error.
 *
 * The resume point is then the end of the last row in the gap-free run from F + 1, or the anchor if there is none.
 */
class RiscvRollupsResumePointProvider(
  private val rollupsDao: RiscvRollupsDao,
  private val riscvCutoverTimestamp: Instant,
  private val genesisDataRollingHashProvider: RiscvGenesisDataRollingHashProvider,
) : RollupResumePointProvider {

  override fun getResumePoint(
    lastFinalizedBlock: ULong,
    lastFinalizedBlockTimestamp: Instant,
  ): SafeFuture<RollupResumePoint> {
    val anchor = if (lastFinalizedBlockTimestamp < riscvCutoverTimestamp) {
      genesisDataRollingHashProvider.getGenesisDataRollingHash()
        .thenApply { RollupResumePoint(lastFinalizedBlock, DaStreamPosition.fresh(it)) }
    } else {
      findRow(lastFinalizedBlock) {
        "L1 finalized block $lastFinalizedBlock is past the RISC-V cutover but no rollup row ends at it"
      }.thenApply(::resumePointAfter)
    }
    return anchor.thenCompose { anchorPoint ->
      rollupsDao.findHighestConsecutiveEndBlockNumberFromBlockNumber(lastFinalizedBlock + 1UL)
        .thenCompose { lastEnd ->
          if (lastEnd == null) {
            SafeFuture.completedFuture(anchorPoint)
          } else {
            findRow(lastEnd) { "no rollup row ends at block $lastEnd" }.thenApply(::resumePointAfter)
          }
        }
    }
  }

  private fun findRow(endBlockNumber: ULong, missingMessage: () -> String): SafeFuture<RiscvRollupRecord> =
    rollupsDao.findRollupByEndBlockNumber(endBlockNumber).thenApply {
      it ?: throw IllegalStateException(missingMessage())
    }

  private fun resumePointAfter(row: RiscvRollupRecord): RollupResumePoint {
    val lastBlob = row.blobsData.last()
    val tail = row.finalSharedBlobCarryOverTailBytes
    return RollupResumePoint(
      lastConflationEndBlock = row.endBlockNumber,
      daPosition = if (tail == 0u) {
        DaStreamPosition.fresh(row.dataRollingHash)
      } else {
        DaStreamPosition(
          dataRollingHash = row.dataRollingHash,
          parentBlobDataRollingHash = lastBlob.parentDataRollingHash,
          sharedBlobCarryOverTailBytes = tail,
          sharedBlob = lastBlob.blobBytes,
        )
      },
    )
  }
}
