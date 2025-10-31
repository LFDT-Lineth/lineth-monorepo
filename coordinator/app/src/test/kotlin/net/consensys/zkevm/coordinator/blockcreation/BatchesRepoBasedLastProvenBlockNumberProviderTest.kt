package net.consensys.zkevm.coordinator.blockcreation

import net.consensys.zkevm.persistence.BatchesRepository
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.kotlin.anyOrNull
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

class BatchesRepoBasedLastProvenBlockNumberProviderTest {
  @Test
  fun returnsStartingBlockNumberAfterInitialization() {
    val batchesRepositoryMock = mock<BatchesRepository>()
    whenever(batchesRepositoryMock.findHighestConsecutiveEndBlockNumberFromBlockNumber(anyOrNull()))
      .thenReturn(SafeFuture.completedFuture(null))
    val startingBlockNumber = 3L
    val provider =
      BatchesRepoBasedLastProvenBlockNumberProvider(
        startingBlockNumber,
        startingBlockNumber - 1L,
        batchesRepositoryMock,
      )
    val initialProvenBlockNumber = provider.getLastProvenBlockNumber()
    assertThat(initialProvenBlockNumber.get()).isEqualTo(startingBlockNumber)
  }

  @Test
  fun returnsFinalizedBlockNumberIfNoDataFromRepository() {
    val batchesRepositoryMock = mock<BatchesRepository>()
    whenever(batchesRepositoryMock.findHighestConsecutiveEndBlockNumberFromBlockNumber(anyOrNull()))
      .thenReturn(SafeFuture.completedFuture(null))
    val startingBlockNumber = 3L
    val provider =
      BatchesRepoBasedLastProvenBlockNumberProvider(
        startingBlockNumber,
        startingBlockNumber - 1L,
        batchesRepositoryMock,
      )
    val newFinalizedBlock = 10L
    provider.updateLatestL1FinalizedBlock(newFinalizedBlock)
    assertThat(provider.getLastProvenBlockNumber().get()).isEqualTo(newFinalizedBlock)
    assertThat(provider.getLatestL1FinalizedBlock()).isEqualTo(newFinalizedBlock)
  }

  @Test
  fun returnsDataFromDbIfThereIsAny() {
    val batchesRepositoryMock = mock<BatchesRepository>()
    val dataInDb = 100L
    whenever(batchesRepositoryMock.findHighestConsecutiveEndBlockNumberFromBlockNumber(anyOrNull()))
      .thenReturn(SafeFuture.completedFuture(dataInDb))
    val startingBlockNumber = 3L
    val provider =
      BatchesRepoBasedLastProvenBlockNumberProvider(
        startingBlockNumber,
        startingBlockNumber - 1L,
        batchesRepositoryMock,
      )
    val newFinalizedBlock = 10L
    provider.updateLatestL1FinalizedBlock(newFinalizedBlock)
    assertThat(provider.getLastProvenBlockNumber().get()).isEqualTo(dataInDb)
    assertThat(provider.getLatestL1FinalizedBlock()).isEqualTo(newFinalizedBlock)
  }

  @Test
  fun savesHighestProvenBlocks() {
    val batchesRepositoryMock = mock<BatchesRepository>()
    val dataInDb = 100L
    whenever(batchesRepositoryMock.findHighestConsecutiveEndBlockNumberFromBlockNumber(anyOrNull()))
      .thenReturn(SafeFuture.completedFuture(dataInDb))
    val startingBlockNumber = 3L
    val provider =
      BatchesRepoBasedLastProvenBlockNumberProvider(
        startingBlockNumber,
        startingBlockNumber - 1L,
        batchesRepositoryMock,
      )
    provider.getLastProvenBlockNumber()
    whenever(batchesRepositoryMock.findHighestConsecutiveEndBlockNumberFromBlockNumber(anyOrNull()))
      .thenReturn(SafeFuture.completedFuture(null))
    assertThat(provider.getLastProvenBlockNumber().get()).isEqualTo(dataInDb)
    assertThat(provider.getLatestL1FinalizedBlock()).isEqualTo(startingBlockNumber - 1L)
  }

  @Test
  fun `getLastKnownProvenBlockNumber doesn't invoke the DB method`() {
    val batchesRepositoryMock = mock<BatchesRepository>()
    val dataInDb = 100L
    whenever(batchesRepositoryMock.findHighestConsecutiveEndBlockNumberFromBlockNumber(anyOrNull()))
      .thenReturn(SafeFuture.completedFuture(dataInDb))
    val startingBlockNumber = 3L
    val provider =
      BatchesRepoBasedLastProvenBlockNumberProvider(
        startingBlockNumber,
        startingBlockNumber - 1L,
        batchesRepositoryMock,
      )
    provider.getLastProvenBlockNumber()
    assertThat(provider.getLastKnownProvenBlockNumber()).isEqualTo(dataInDb)
    assertThat(provider.getLatestL1FinalizedBlock()).isEqualTo(startingBlockNumber - 1L)

    verify(batchesRepositoryMock, times(1)).findHighestConsecutiveEndBlockNumberFromBlockNumber(anyOrNull())
  }
}
