package lineth.coordinator.app

import lineth.coordinator.app.conflation.ConflationAppHelper.cleanupDbDataAfterBlockNumbers
import lineth.coordinator.app.conflation.ConflationAppHelper.forcedTransactionsEnabled
import lineth.coordinator.app.conflation.ConflationAppHelper.resumeConflationFrom
import lineth.coordinator.config.v2.ForcedTransactionsConfig
import lineth.coordinator.config.v2.L1SubmissionConfig
import lineth.persistence.AggregationsRepository
import lineth.persistence.BatchesRepository
import lineth.persistence.BlobsRepository
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.Mockito.anyLong
import org.mockito.Mockito.mock
import org.mockito.Mockito.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.net.URI

class ConflationAppTest {
  @Test
  fun `forced transactions are enabled only when configured and not on validium`() {
    val enabledConfig = ForcedTransactionsConfig(
      disabled = false,
      l1Endpoint = URI.create("http://l1").toURL(),
      sequencerEndpoint = URI.create("http://sequencer").toURL(),
    )
    val rollup = L1SubmissionConfig.DataAvailability.ROLLUP
    val validium = L1SubmissionConfig.DataAvailability.VALIDIUM

    assertThat(forcedTransactionsEnabled(null, rollup)).isFalse()
    assertThat(forcedTransactionsEnabled(enabledConfig.copy(disabled = true), rollup)).isFalse()
    assertThat(forcedTransactionsEnabled(enabledConfig, validium)).isFalse()
    assertThat(forcedTransactionsEnabled(enabledConfig, rollup)).isTrue()
  }

  @Test
  fun `test resume conflation from uses lastFinalizedBlock + 1 for db queries`() {
    val aggregationsRepository = mock<AggregationsRepository>()
    val lastFinalizedBlock = 100uL

    whenever(aggregationsRepository.findConsecutiveProvenBlobs(101L))
      .thenReturn(SafeFuture.completedFuture(emptyList()))

    val lastProcessedBlock =
      resumeConflationFrom(
        aggregationsRepository,
        lastFinalizedBlock,
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

    cleanupDbDataAfterBlockNumbers(
      lastProcessedBlockNumber = lastProcessedBlock,
      lastConsecutiveAggregatedBlockNumber = lastConsecutiveAggregatedBlockNumber,
      batchesRepository = batchesRepository,
      blobsRepository = blobsRepository,
      aggregationsRepository = aggregationsRepository,
    ).get()
    verify(batchesRepository).deleteBatchesAfterBlockNumber((lastProcessedBlock + 1uL).toLong())
    verify(blobsRepository).deleteBlobsAfterBlockNumber(lastProcessedBlock + 1uL)
    verify(aggregationsRepository).deleteAggregationsAfterBlockNumber(
      (lastConsecutiveAggregatedBlockNumber + 1uL).toLong(),
    )
  }
}
