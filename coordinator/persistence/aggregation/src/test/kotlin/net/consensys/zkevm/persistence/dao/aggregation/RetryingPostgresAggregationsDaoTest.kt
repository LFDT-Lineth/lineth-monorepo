package net.consensys.zkevm.persistence.dao.aggregation

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import kotlinx.datetime.Instant
import linea.domain.BlockIntervals
import net.consensys.FakeFixedClock
import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.domain.blobCounters
import net.consensys.zkevm.domain.createAggregation
import net.consensys.zkevm.domain.createProofToFinalize
import net.consensys.zkevm.persistence.db.PersistenceRetryer
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito.mock
import org.mockito.Mockito.times
import org.mockito.kotlin.eq
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class RetryingPostgresAggregationsDaoTest {
  private lateinit var retryingAggregationsPostgresDao: RetryingPostgresAggregationsDao
  private val delegateAggregationsDao = mock<PostgresAggregationsDao>()
  private val fakeClock = FakeFixedClock()
  private val createdBeforeInstant = fakeClock.now()
  private val aggregation = createAggregation(
    startBlockNumber = 1,
    endBlockNumber = 10
  )

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    retryingAggregationsPostgresDao = RetryingPostgresAggregationsDao(
      delegate = delegateAggregationsDao,
      PersistenceRetryer(
        vertx = vertx,
        PersistenceRetryer.Config(
          backoffDelay = 1.milliseconds
        )
      )
    )

    val baseBlobCounters = blobCounters(startBlockNumber = 1uL, endBlockNumber = 10uL)
    val baseBlobAndBatchCounters = BlobAndBatchCounters(
      blobCounters = baseBlobCounters,
      executionProofs = BlockIntervals(1UL, listOf(2UL, 3UL))
    )

    val aggregationProof = createProofToFinalize(
      firstBlockNumber = 0,
      finalBlockNumber = 10,
      parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(0),
      startBlockTime = Instant.fromEpochSeconds(0),
      finalTimestamp = Instant.parse("2024-04-28T15:00:00Z")
    )

    whenever(delegateAggregationsDao.findConsecutiveProvenBlobs(eq(0L)))
      .thenReturn(SafeFuture.completedFuture(listOf(baseBlobAndBatchCounters)))

    whenever(delegateAggregationsDao.saveNewAggregation(eq(aggregation)))
      .thenReturn(SafeFuture.completedFuture(Unit))

    whenever(
      delegateAggregationsDao.getProofsToFinalize(
        eq(0L),
        eq(createdBeforeInstant),
        eq(1)
      )
    )
      .thenReturn(SafeFuture.completedFuture(listOf(aggregationProof)))

    whenever(delegateAggregationsDao.findAggregationProofByEndBlockNumber(eq(10L)))
      .thenReturn(SafeFuture.completedFuture(aggregationProof))

    whenever(delegateAggregationsDao.deleteAggregationsUpToEndBlockNumber(eq(10L)))
      .thenReturn(SafeFuture.completedFuture(1))

    whenever(delegateAggregationsDao.deleteAggregationsAfterBlockNumber(eq(0L)))
      .thenReturn(SafeFuture.completedFuture(1))
  }

  @Test
  fun `retrying batches dao should delegate all queries to standard dao`() {
    retryingAggregationsPostgresDao.findConsecutiveProvenBlobs(0L)
    verify(delegateAggregationsDao, times(1)).findConsecutiveProvenBlobs(eq(0L))

    retryingAggregationsPostgresDao.saveNewAggregation(aggregation)
    verify(delegateAggregationsDao, times(1)).saveNewAggregation(eq(aggregation))

    retryingAggregationsPostgresDao.getProofsToFinalize(
      fromBlockNumber = 0L,
      finalEndBlockCreatedBefore = createdBeforeInstant,
      maximumNumberOfProofs = 1
    )
    verify(delegateAggregationsDao, times(1)).getProofsToFinalize(
      eq(0L),
      eq(createdBeforeInstant),
      eq(1)
    )

    retryingAggregationsPostgresDao.findAggregationProofByEndBlockNumber(10L)
    verify(delegateAggregationsDao, times(1)).findAggregationProofByEndBlockNumber(eq(10L))

    retryingAggregationsPostgresDao.deleteAggregationsUpToEndBlockNumber(10L)
    verify(delegateAggregationsDao, times(1)).deleteAggregationsUpToEndBlockNumber(eq(10L))

    retryingAggregationsPostgresDao.deleteAggregationsAfterBlockNumber(0L)
    verify(delegateAggregationsDao, times(1)).deleteAggregationsAfterBlockNumber(eq(0L))
  }
}
