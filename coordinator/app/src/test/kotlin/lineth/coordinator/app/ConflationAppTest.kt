package lineth.coordinator.app

import linea.clients.L2ExecutionProverClientV1
import linea.clients.RollupAggregationProverClientV1
import linea.clients.RollupProverClientV1
import linea.domain.BlockParameter
import linea.domain.createBlock
import linea.domain.toBlockWithRandomTxHashes
import linea.ethapi.EthApiClient
import lineth.coordinator.app.conflation.ConflationAppHelper.cleanupDbDataAfterBlockNumbers
import lineth.coordinator.app.conflation.ConflationAppHelper.cleanupRiscvDbDataAfterBlockNumbers
import lineth.coordinator.app.conflation.ConflationAppHelper.cleanupRiscvProverRequests
import lineth.coordinator.app.conflation.ConflationAppHelper.forcedTransactionsEnabled
import lineth.coordinator.app.conflation.ConflationAppHelper.getLastRiscvProcessedBlocks
import lineth.coordinator.app.conflation.ConflationAppHelper.resumeConflationFrom
import lineth.coordinator.app.conflation.ConflationAppHelper.resumeRiscvRollupFrom
import lineth.coordinator.config.v2.ForcedTransactionsConfig
import lineth.coordinator.config.v2.L1SubmissionConfig
import lineth.persistence.AggregationsRepository
import lineth.persistence.BatchesRepository
import lineth.persistence.BlobsRepository
import lineth.persistence.BlobsRepositoryV2
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.mockito.Mockito.anyLong
import org.mockito.Mockito.mock
import org.mockito.Mockito.verify
import org.mockito.kotlin.any
import org.mockito.kotlin.never
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

  private fun l2ClientReturningBlocks(): EthApiClient {
    val l2EthClient = mock<EthApiClient>()
    whenever(l2EthClient.ethGetBlockByNumberTxHashes(any())).thenAnswer {
      val number = (it.arguments[0] as BlockParameter.BlockNumber).number
      SafeFuture.completedFuture(createBlock(number = number).toBlockWithRandomTxHashes())
    }
    return l2EthClient
  }

  @Test
  fun `riscv pipeline resumes after the consecutive batches and rollup rows, aggregating from the finalized block`() {
    val batchesRepository = mock<BatchesRepository>()
    val blobsRepository = mock<BlobsRepositoryV2>()
    whenever(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(101L))
      .thenReturn(SafeFuture.completedFuture(150L))
    whenever(blobsRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(101uL))
      .thenReturn(SafeFuture.completedFuture(140uL))

    val lastProcessedBlocks =
      getLastRiscvProcessedBlocks(100uL, batchesRepository, blobsRepository, l2ClientReturningBlocks()).get()

    assertThat(lastProcessedBlocks.lastConflatedBlock.number).isEqualTo(150uL)
    assertThat(lastProcessedBlocks.lastRollupEndBlockNumber).isEqualTo(140uL)
    assertThat(lastProcessedBlocks.lastAggregatedBlock.number).isEqualTo(100uL)
  }

  @Test
  fun `riscv pipeline resumes at the finalized block when no batch or rollup row follows it`() {
    val batchesRepository = mock<BatchesRepository>()
    val blobsRepository = mock<BlobsRepositoryV2>()
    whenever(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(101L))
      .thenReturn(SafeFuture.completedFuture(null))
    whenever(blobsRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(101uL))
      .thenReturn(SafeFuture.completedFuture(null))

    val lastProcessedBlocks =
      getLastRiscvProcessedBlocks(100uL, batchesRepository, blobsRepository, l2ClientReturningBlocks()).get()

    assertThat(lastProcessedBlocks.lastConflatedBlock.number).isEqualTo(100uL)
    assertThat(lastProcessedBlocks.lastRollupEndBlockNumber).isEqualTo(100uL)
    assertThat(lastProcessedBlocks.lastAggregatedBlock.number).isEqualTo(100uL)
  }

  @Test
  fun `riscv rollups resume after the consecutive rollup rows, or at the finalized block`() {
    val blobsRepository = mock<BlobsRepositoryV2>()
    whenever(blobsRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(101uL))
      .thenReturn(SafeFuture.completedFuture(140uL))
    whenever(blobsRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(201uL))
      .thenReturn(SafeFuture.completedFuture(null))

    assertThat(resumeRiscvRollupFrom(blobsRepository, 100uL).get()).isEqualTo(140uL)
    assertThat(resumeRiscvRollupFrom(blobsRepository, 200uL).get()).isEqualTo(200uL)
  }

  @Test
  fun `riscv startup db cleanup deletes after each stage's resume point`() {
    val batchesRepository = mock<BatchesRepository>()
    val blobsRepository = mock<BlobsRepositoryV2>()
    val aggregationsRepository = mock<AggregationsRepository>()
    whenever(batchesRepository.deleteBatchesAfterBlockNumber(anyLong()))
      .thenReturn(SafeFuture.completedFuture(0))
    whenever(blobsRepository.deleteBlobsAfterBlockNumber(anyLong().toULong()))
      .thenReturn(SafeFuture.completedFuture(0))
    whenever(aggregationsRepository.deleteAggregationsAfterBlockNumber(anyLong()))
      .thenReturn(SafeFuture.completedFuture(0))

    cleanupRiscvDbDataAfterBlockNumbers(
      lastConflatedBlockNumber = 150uL,
      lastRollupEndBlockNumber = 140uL,
      lastAggregatedBlockNumber = 100uL,
      batchesRepository = batchesRepository,
      blobsRepository = blobsRepository,
      aggregationsRepository = aggregationsRepository,
    ).get()

    verify(batchesRepository).deleteBatchesAfterBlockNumber(151L)
    verify(blobsRepository).deleteBlobsAfterBlockNumber(141uL)
    verify(aggregationsRepository).deleteAggregationsAfterBlockNumber(101L)
  }

  @Test
  fun `riscv startup db cleanup fails when the last rollup ends after the last persisted batch`() {
    val batchesRepository = mock<BatchesRepository>()

    assertThatThrownBy {
      cleanupRiscvDbDataAfterBlockNumbers(
        lastConflatedBlockNumber = 150uL,
        lastRollupEndBlockNumber = 160uL,
        lastAggregatedBlockNumber = 100uL,
        batchesRepository = batchesRepository,
        blobsRepository = mock<BlobsRepositoryV2>(),
        aggregationsRepository = mock<AggregationsRepository>(),
      ).get()
    }.hasMessageContaining("last rollup ends at block 160, after the last persisted batch 150")
    verify(batchesRepository, never()).deleteBatchesAfterBlockNumber(anyLong())
  }

  @Test
  fun `riscv startup request cleanup removes requests after each stage's resume point`() {
    val l2ExecutionProverClient = mock<L2ExecutionProverClientV1>()
    val rollupProverClient = mock<RollupProverClientV1>()
    val rollupAggregationProverClient = mock<RollupAggregationProverClientV1>()
    listOf(l2ExecutionProverClient, rollupProverClient, rollupAggregationProverClient).forEach {
      whenever(it.removeRequests(any())).thenReturn(SafeFuture.completedFuture(Unit))
    }

    cleanupRiscvProverRequests(
      l2ExecutionProverClient = l2ExecutionProverClient,
      rollupProverClient = rollupProverClient,
      rollupAggregationProverClient = rollupAggregationProverClient,
      lastConflatedBlockNumber = 150uL,
      lastRollupEndBlockNumber = 140uL,
      lastAggregatedBlockNumber = 100uL,
    ).get()

    verify(l2ExecutionProverClient).removeRequests(151L)
    verify(rollupProverClient).removeRequests(141L)
    verify(rollupAggregationProverClient).removeRequests(101L)
  }
}
