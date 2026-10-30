package lineth.persistence.conflation

import linea.clients.RollupAggregationProofResponseV1
import linea.domain.AggregationG
import linea.domain.BlobAndBatchCounters
import linea.domain.ProofToFinalize
import linea.error.DuplicatedRecordException
import lineth.persistence.AggregationsDao
import lineth.persistence.AggregationsDaoG
import lineth.persistence.AggregationsDaoV2
import lineth.persistence.AggregationsRepository
import lineth.persistence.AggregationsRepositoryG
import lineth.persistence.AggregationsRepositoryV2
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

abstract class AggregationsRepositoryImplG<T>(
  private val aggregationsDao: AggregationsDaoG<T>,
) : AggregationsRepositoryG<T> {

  override fun saveNewAggregation(aggregation: AggregationG<T>): SafeFuture<Unit> {
    return aggregationsDao.saveNewAggregation(aggregation)
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
  ): SafeFuture<List<T>> {
    return aggregationsDao.getProofsToFinalize(
      fromBlockNumber,
      finalEndBlockCreatedBefore,
      maximumNumberOfProofs,
    )
  }

  override fun findHighestConsecutiveEndBlockNumber(fromBlockNumber: Long?): SafeFuture<Long?> {
    return aggregationsDao.findHighestConsecutiveEndBlockNumber(fromBlockNumber)
  }

  override fun findAggregationProofByEndBlockNumber(endBlockNumber: Long): SafeFuture<T?> {
    return aggregationsDao.findAggregationProofByEndBlockNumber(endBlockNumber)
  }

  override fun deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive: Long): SafeFuture<Int> {
    return aggregationsDao.deleteAggregationsUpToEndBlockNumber(endBlockNumberInclusive)
  }

  override fun deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive: Long): SafeFuture<Int> {
    return aggregationsDao.deleteAggregationsAfterBlockNumber(startingBlockNumberInclusive)
  }
}

class AggregationsRepositoryImpl(
  private val aggregationsDao: AggregationsDao,
) : AggregationsRepositoryImplG<ProofToFinalize>(aggregationsDao), AggregationsRepository {

  override fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>> {
    return aggregationsDao.findConsecutiveProvenBlobs(fromBlockNumber)
  }
}

class AggregationsRepositoryImplV2(
  aggregationsDao: AggregationsDaoV2,
) : AggregationsRepositoryImplG<RollupAggregationProofResponseV1>(aggregationsDao), AggregationsRepositoryV2
