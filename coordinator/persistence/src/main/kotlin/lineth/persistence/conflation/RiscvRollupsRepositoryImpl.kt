package lineth.persistence.conflation

import linea.domain.RiscvRollupRecord
import linea.error.DuplicatedRecordException
import lineth.persistence.RiscvRollupsRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

class RiscvRollupsRepositoryImpl(
  private val rollupsDao: RiscvRollupsDao,
) : RiscvRollupsRepository {
  override fun saveRollupWithBlobs(rollup: RiscvRollupRecord): SafeFuture<Unit> =
    rollupsDao.saveRollupWithBlobs(rollup)
      .exceptionallyCompose { error ->
        if (error is DuplicatedRecordException) {
          SafeFuture.completedFuture(Unit)
        } else {
          SafeFuture.failedFuture(error)
        }
      }

  override fun getConsecutiveRollupsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<RiscvRollupRecord>> =
    rollupsDao.getConsecutiveRollupsFromBlockNumber(startingBlockNumberInclusive, endBlockCreatedBefore)

  override fun findRollupByStartBlockNumber(startBlockNumber: ULong): SafeFuture<RiscvRollupRecord?> =
    rollupsDao.findRollupByStartBlockNumber(startBlockNumber)

  override fun findRollupByEndBlockNumber(endBlockNumber: ULong): SafeFuture<RiscvRollupRecord?> =
    rollupsDao.findRollupByEndBlockNumber(endBlockNumber)

  override fun deleteRollupsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> =
    rollupsDao.deleteRollupsUpToEndBlockNumber(endBlockNumberInclusive)

  override fun deleteRollupsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> =
    rollupsDao.deleteRollupsAfterBlockNumber(startingBlockNumberInclusive)
}
