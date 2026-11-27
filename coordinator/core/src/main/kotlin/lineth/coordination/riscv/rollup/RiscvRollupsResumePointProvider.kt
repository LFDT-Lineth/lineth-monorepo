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
 * Finds where the rollup stage resumes after a restart: right after the newest rollup that is persisted and continues
 * the L1 finalized block F without gaps, so rollups already proven are not proven again.
 *
 * 1. Walk the persisted rollups from F + 1: the one starting at F + 1, the one starting right after it, and so on, up
 *    to the first gap. Rollups after a gap are not used.
 * 2. If the walk finds rollups, resume after the last one: the next rollup starts at its end block + 1, and the DA
 *    stream reopens where it ended, inside its last blob if it left unused bytes there, else on a fresh blob.
 * 3. If no persisted rollup starts at F + 1, resume right after F, with the DA stream position at F:
 *    - F before the RISC-V cutover, or F the genesis block: no RISC-V rollup is finalized yet, so the stream starts
 *      fresh from the genesis data rolling hash. Before the cutover, this assumes the pre-RISC-V flow has finalized
 *      every pre-RISC-V block, so F is the last one. The genesis block is never in a rollup, so a chain that is
 *      RISC-V from genesis starts there.
 *    - Otherwise: the position is read from the persisted rollup ending at F, which finalization cleanup keeps for
 *      this. If it is missing, this fails.
 */
class RiscvRollupsResumePointProvider(
  private val rollupsDao: RiscvRollupsDao,
  private val riscvCutoverTimestamp: Instant,
  private val genesisDataRollingHashProvider: RiscvGenesisDataRollingHashProvider,
) : RollupResumePointProvider {

  override fun getResumePoint(
    lastFinalizedBlock: ULong,
    lastFinalizedBlockTimestamp: Instant,
  ): SafeFuture<RollupResumePoint> =
    rollupsDao.findHighestConsecutiveRollupFromBlockNumber(lastFinalizedBlock + 1UL)
      .thenCompose { lastRollup ->
        when {
          lastRollup != null -> SafeFuture.completedFuture(resumePointAfter(lastRollup))

          lastFinalizedBlock == 0UL || lastFinalizedBlockTimestamp < riscvCutoverTimestamp ->
            genesisDataRollingHashProvider.getGenesisDataRollingHash()
              .thenApply { RollupResumePoint(lastFinalizedBlock, DaStreamPosition.fresh(it)) }

          else -> getRollupByEndBlockNumber(lastFinalizedBlock).thenApply(::resumePointAfter)
        }
      }

  private fun getRollupByEndBlockNumber(endBlockNumber: ULong): SafeFuture<RiscvRollupRecord> =
    rollupsDao.findRollupByEndBlockNumber(endBlockNumber).thenApply {
      it ?: throw IllegalStateException(
        "L1 finalized block $endBlockNumber is past the RISC-V cutover but no rollup row ends at it",
      )
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
