package net.consensys.zkevm.coordinator.app

import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BatchesRepository
import net.consensys.zkevm.persistence.BlobsRepository
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.Mockito.anyLong
import org.mockito.Mockito.mock
import org.mockito.Mockito.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

class L1DependentAppTest {
  @Test
  fun `test resume conflation from uses lastFinalizedBlock + 1 for db queries`() {
    val aggregationsRepository = mock<AggregationsRepository>()
    val lastFinalizedBlock = 100uL

    whenever(aggregationsRepository.findConsecutiveProvenBlobs(101L))
      .thenReturn(SafeFuture.completedFuture(emptyList()))

    val lastProcessedBlock =
      L1DependentApp.resumeConflationFrom(
        aggregationsRepository,
        lastFinalizedBlock
      ).get()
    assertThat(lastProcessedBlock).isEqualTo(lastFinalizedBlock)
    verify(aggregationsRepository).findConsecutiveProvenBlobs(lastFinalizedBlock.toLong() + 1)
  }

  @Test
  fun `test startup db cleanup use lastProcessedBlock + 1 for cleaning objects`() {
    val batchesRepository = mock<BatchesRepository>()
    val blobsRepository = mock<BlobsRepository>()
    val aggregationsRepository = mock<AggregationsRepository>()
    val lastProcessedBlock = 100uL
    val lastConsecutiveAggregatedBlockNumber = 80uL

    whenever(batchesRepository.deleteBatchesAfterBlockNumber(anyLong()))
      .thenReturn(SafeFuture.completedFuture(0))
    whenever(blobsRepository.deleteBlobsAfterBlockNumber(anyLong().toULong()))
      .thenReturn(SafeFuture.completedFuture(0))
    whenever(aggregationsRepository.deleteAggregationsAfterBlockNumber(anyLong()))
      .thenReturn(SafeFuture.completedFuture(0))

    L1DependentApp.cleanupDbDataAfterBlockNumbers(
      lastProcessedBlockNumber = lastProcessedBlock,
      lastConsecutiveAggregatedBlockNumber = lastConsecutiveAggregatedBlockNumber,
      batchesRepository = batchesRepository,
      blobsRepository = blobsRepository,
      aggregationsRepository = aggregationsRepository
    ).get()
    verify(batchesRepository).deleteBatchesAfterBlockNumber((lastProcessedBlock + 1uL).toLong())
    verify(blobsRepository).deleteBlobsAfterBlockNumber(lastProcessedBlock + 1uL)
    verify(aggregationsRepository).deleteAggregationsAfterBlockNumber(
      (lastConsecutiveAggregatedBlockNumber + 1uL).toLong()
    )
  }
}
