package lineth.persistence.conflation

import linea.clients.RollupAggregationProofResponseV1
import linea.domain.AggregationG
import linea.domain.BlobAndBatchCounters
import linea.domain.ProofToFinalize
import linea.persistence.db.PersistenceRetryer
import lineth.persistence.AggregationsDao
import lineth.persistence.AggregationsDaoG
import lineth.persistence.AggregationsDaoV2
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

abstract class RetryingPostgresAggregationsDaoG<T>(
  private val delegate: AggregationsDaoG<T>,
  protected val persistenceRetryer: PersistenceRetryer,
) : AggregationsDaoG<T> {

  override fun saveNewAggregation(aggregation: AggregationG<T>): SafeFuture<Unit> {
    return delegate.saveNewAggregation(aggregation)
  }

  override fun getProofsToFinalize(
    fromBlockNumber: Long,
    finalEndBlockCreatedBefore: Instant,
    maximumNumberOfProofs: Int,
  ): SafeFuture<List<T>> {
    return persistenceRetryer.retryQuery(
      {
        delegate.getProofsToFinalize(
          fromBlockNumber,
          finalEndBlockCreatedBefore,
          maximumNumberOfProofs,
        )
      },
    )
  }

  override fun findHighestConsecutiveEndBlockNumber(fromBlockNumber: Long?): SafeFuture<Long?> {
    return persistenceRetryer.retryQuery(
      { delegate.findHighestConsecutiveEndBlockNumber(fromBlockNumber) },
    )
  }

  override fun findAggregationProofByEndBlockNumber(endBlockNumber: Long): SafeFuture<T?> {
    return persistenceRetryer.retryQuery({ delegate.findAggregationProofByEndBlockNumber(endBlockNumber) })
  }

  override fun deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive) })
  }

  override fun deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int> {
    return persistenceRetryer.retryQuery({ delegate.deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive) })
  }
}

class RetryingPostgresAggregationsDao(
  private val delegate: PostgresAggregationsDao,
  persistenceRetryer: PersistenceRetryer,
) : RetryingPostgresAggregationsDaoG<ProofToFinalize>(delegate, persistenceRetryer), AggregationsDao {

  override fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>> {
    return persistenceRetryer.retryQuery(
      { delegate.findConsecutiveProvenBlobs(fromBlockNumber) },
    )
  }
}

class RetryingPostgresAggregationsDaoV2(
  delegate: PostgresAggregationsDaoV2,
  persistenceRetryer: PersistenceRetryer,
) : RetryingPostgresAggregationsDaoG<RollupAggregationProofResponseV1>(delegate, persistenceRetryer), AggregationsDaoV2
