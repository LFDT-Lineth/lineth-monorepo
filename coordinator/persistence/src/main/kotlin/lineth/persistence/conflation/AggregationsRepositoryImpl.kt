package lineth.persistence.conflation

import lineth.persistence.AggregationsDao
import lineth.persistence.AggregationsRepository
import lineth.domain.Aggregation
import lineth.domain.BlobAndBatchCounters
import lineth.domain.ProofToFinalize
import lineth.error.DuplicatedRecordException
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

class AggregationsRepositoryImpl(
  private val aggregationsPostgresDao: AggregationsDao,
) : AggregationsRepository {
  override fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>> {
    return aggregationsPostgresDao.findConsecutiveProvenBlobs(fromBlockNumber)
  }

  override fun saveNewAggregation(aggregation: Aggregation): SafeFuture<Unit> {
    return aggregationsPostgresDao.saveNewAggregation(aggregation)
      .exceptionallyCompose { error ->
        if (error is DuplicatedRecordException) {
          SafeFuture.completedFuture(Unit)
        } else {
          SafeFuture.failedFuture(error)
        }
      }
  }

  override fun getProofsToFinalize(
    fromBlockNumber: Long,
    finalEndBlockCreatedBefore: Instant,
    maximumNumberOfProofs: Int,
  ): SafeFuture<List<ProofToFinalize>> {
    return aggregationsPostgresDao.getProofsToFinalize(
      fromBlockNumber,
      finalEndBlockCreatedBefore,
      maximumNumberOfProofs,
    )
  }

  override fun findHighestConsecutiveEndBlockNumber(fromBlockNumber: Long?): SafeFuture<Long?> {
    return aggregationsPostgresDao.findHighestConsecutiveEndBlockNumber(fromBlockNumber)
  }

  override fun findAggregationProofByEndBlockNumber(endBlockNumber: Long): SafeFuture<ProofToFinalize?> {
    return aggregationsPostgresDao.findAggregationProofByEndBlockNumber(endBlockNumber)
  }

  override fun deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int> {
    return aggregationsPostgresDao.deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive)
  }

  override fun deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int> {
    return aggregationsPostgresDao.deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive)
  }
}
