package net.consensys.zkevm.persistence.dao.aggregation

import linea.domain.Aggregation
import linea.domain.BlobAndBatchCounters
import linea.domain.ProofToFinalize
import linea.error.DuplicatedRecordException
import linea.persistence.AggregationsDao
import linea.persistence.AggregationsRepository
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
