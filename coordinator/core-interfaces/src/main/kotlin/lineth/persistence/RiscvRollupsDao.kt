package lineth.persistence

import linea.domain.RiscvRollupRecord
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

/**
 * RISC-V rollup proofs and their blobs. A blob shared by adjacent rollups is stored once; deleting rollups also
 * deletes the blobs no remaining rollup uses.
 */
interface RiscvRollupsDao {
  /**
   * Saves [rollup] and its blobs. Saving a rollup that is already stored is a duplicate: ignored or failed with
   * `DuplicatedRecordException`, depending on the implementation's configuration. A rollup that shares only its start
   * or end block with a stored one always fails.
   */
  fun saveRollupWithBlobs(rollup: RiscvRollupRecord): SafeFuture<Unit>

  /**
   * The gap-free run of rollups starting at [startingBlockNumberInclusive], keeping those whose last block's timestamp
   * is before [endBlockCreatedBefore].
   */
  fun getConsecutiveRollupsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<RiscvRollupRecord>>

  fun findRollupByStartBlockNumber(startBlockNumber: ULong): SafeFuture<RiscvRollupRecord?>

  fun findRollupByEndBlockNumber(endBlockNumber: ULong): SafeFuture<RiscvRollupRecord?>

  fun deleteRollupsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int>

  fun deleteRollupsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int>
}
