package lineth.persistence.conflation

import linea.domain.RiscvRollupRecord
import linea.persistence.db.PersistenceRetryer
import lineth.persistence.RiscvRollupsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

class RetryingRiscvRollupsPostgresDao(
  private val delegate: RiscvRollupsDao,
  private val persistenceRetryer: PersistenceRetryer,
) : RiscvRollupsDao {
  override fun saveRollupWithBlobs(rollup: RiscvRollupRecord): SafeFuture<Unit> =
    persistenceRetryer.retryQuery({ delegate.saveRollupWithBlobs(rollup) })

  override fun getConsecutiveRollupsFromBlockNumber(
    startingBlockNumberInclusive: ULong,
    endBlockCreatedBefore: Instant,
  ): SafeFuture<List<RiscvRollupRecord>> =
    persistenceRetryer.retryQuery({
      delegate.getConsecutiveRollupsFromBlockNumber(startingBlockNumberInclusive, endBlockCreatedBefore)
    })

  override fun findRollupByStartBlockNumber(startBlockNumber: ULong): SafeFuture<RiscvRollupRecord?> =
    persistenceRetryer.retryQuery({ delegate.findRollupByStartBlockNumber(startBlockNumber) })

  override fun findRollupByEndBlockNumber(endBlockNumber: ULong): SafeFuture<RiscvRollupRecord?> =
    persistenceRetryer.retryQuery({ delegate.findRollupByEndBlockNumber(endBlockNumber) })

  override fun deleteRollupsUpToEndBlockNumber(endBlockNumberInclusive: ULong): SafeFuture<Int> =
    persistenceRetryer.retryQuery({ delegate.deleteRollupsUpToEndBlockNumber(endBlockNumberInclusive) })

  override fun deleteRollupsAfterBlockNumber(startingBlockNumberInclusive: ULong): SafeFuture<Int> =
    persistenceRetryer.retryQuery({ delegate.deleteRollupsAfterBlockNumber(startingBlockNumberInclusive) })
}
