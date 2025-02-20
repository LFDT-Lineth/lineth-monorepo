package net.consensys.zkevm.ethereum.coordination.aggregation

import kotlinx.datetime.Instant
import linea.domain.BlockIntervals
import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.persistence.AggregationsRepository
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random

class ConsecutiveProvenBlobsProviderWithLastEndBlockNumberTrackerTest {
  @Test
  fun `findConsecutiveProvenBlobs the cache saves the last seen end block number`() {
    val repositoryMock = mock<AggregationsRepository>()
    val cache = ConsecutiveProvenBlobsProviderWithLastEndBlockNumberTracker(repositoryMock, 12UL)

    assertThat(cache.get()).isEqualTo(12L)

    val baseBlobCounters = BlobCounters(
      numberOfBatches = 1U,
      startBlockNumber = 1UL,
      endBlockNumber = 2UL,
      startBlockTimestamp = Instant.DISTANT_PAST,
      endBlockTimestamp = Instant.DISTANT_PAST,
      expectedShnarf = Random.nextBytes(32)
    )
    val baseBlobAndBatchCounters = BlobAndBatchCounters(
      blobCounters = baseBlobCounters,
      executionProofs = BlockIntervals(1UL, listOf(2UL, 3UL))
    )
    val expectedEndBLockNumber = 10UL
    val lastBlobAndBatchCounters = baseBlobAndBatchCounters.copy(
      blobCounters = baseBlobCounters.copy(startBlockNumber = 3UL, endBlockNumber = expectedEndBLockNumber)
    )
    whenever(repositoryMock.findConsecutiveProvenBlobs(any())).thenReturn(
      SafeFuture.completedFuture(
        listOf(
          baseBlobAndBatchCounters,
          lastBlobAndBatchCounters
        )
      )
    )

    cache.findConsecutiveProvenBlobs(expectedEndBLockNumber.toLong())
    assertThat(cache.get()).isEqualTo(expectedEndBLockNumber.toLong())
  }
}
