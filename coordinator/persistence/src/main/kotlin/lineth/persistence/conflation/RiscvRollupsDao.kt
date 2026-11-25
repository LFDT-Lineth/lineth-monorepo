package lineth.persistence.conflation

import linea.domain.RiscvRollupRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

/** RISC-V rollup proofs and their blobs; see [lineth.persistence.RiscvRollupsRepository]. */
interface RiscvRollupsDao {
  /** Saves [rollup] and its blobs; fails with `DuplicatedRecordException` if the rollup is already saved. */
  fun saveRollupWithBlobs(rollup: RiscvRollupRecord): SafeFuture<Unit>

  fun getConsecutiveRollupsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<RiscvRollupRecord>>

  fun findRollupByStartBlockNumber(startBlockNumber: ULong): SafeFuture<RiscvRollupRecord?>

  fun findRollupByEndBlockNumber(endBlockNumber: ULong): SafeFuture<RiscvRollupRecord?>

  fun deleteRollupsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int>

  fun deleteRollupsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int>
}
