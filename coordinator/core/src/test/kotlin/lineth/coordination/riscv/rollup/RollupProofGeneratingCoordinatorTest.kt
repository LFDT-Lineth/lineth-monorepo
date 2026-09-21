package lineth.coordination.riscv.rollup

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.RollupProverClientV1
import linea.domain.Batch
import linea.domain.BlockIntervalProofIndex
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.Constants
import linea.domain.DataRollingHashCalculator
import linea.domain.StreamPosition
import linea.domain.createBlock
import lineth.coordination.riscv.conflation.ConflationSegmentBuilder
import lineth.encoding.BlockEncoder
import lineth.persistence.BatchesRepository
import net.consensys.linea.traces.TracesCountersV2
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.argumentCaptor
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

@ExtendWith(VertxExtension::class)
class RollupProofGeneratingCoordinatorTest {

  private val chainId = 1UL
  private val conflationsPerRollupProof = 2
  private val genesisDataRollingHash = ByteArray(32) { 0 }
  private val segmentSize = 100
  private val fakeSegment = ByteArray(segmentSize) { it.toByte() }
  private val fakeEncodedBlock = ByteArray(10) { 0xAB.toByte() }
  private val fakeChunkHash = ByteArray(32) { 0x42.toByte() }

  private val rollupProverClient = mock<RollupProverClientV1>()
  private val batchesRepository = mock<BatchesRepository>()
  private val rollupProofPoller = mock<RollupProofPoller>()
  private val blockEncoder = mock<BlockEncoder>()

  private val conflationSegmentBuilder = ConflationSegmentBuilder { _, _ -> fakeSegment }
  private val dataRollingHashCalculator = DataRollingHashCalculator { _, chunk -> chunk.copyOf(32) }
  private val chunkHasher: (ByteArray) -> ByteArray = { _ -> fakeChunkHash }

  private lateinit var coordinator: RollupProofGeneratingCoordinator

  @BeforeEach
  fun setUp(vertx: Vertx) {
    whenever(blockEncoder.encode(any())).thenReturn(fakeEncodedBlock)

    coordinator = RollupProofGeneratingCoordinator(
      chainId = chainId,
      conflationsPerRollupProof = conflationsPerRollupProof,
      rollupProverClient = rollupProverClient,
      batchesRepository = batchesRepository,
      streamPositionProvider = { SafeFuture.completedFuture(StreamPosition(0UL, genesisDataRollingHash, 0)) },
      rollupProofPoller = rollupProofPoller,
      conflationSegmentBuilder = conflationSegmentBuilder,
      dataRollingHashCalculator = dataRollingHashCalculator,
      blockEncoder = blockEncoder,
      chunkHasher = chunkHasher,
      proofCheckingInterval = 1.seconds,
      vertx = vertx,
      metricsFacade = mock(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    )
    coordinator.start().get()
  }

  private fun makeConflation(start: ULong, end: ULong): BlocksConflation {
    val blocks = (start..end).map { blockNum ->
      createBlock(number = blockNum, timestamp = Instant.fromEpochSeconds(blockNum.toLong() * 12))
    }
    return BlocksConflation(
      blocks = blocks,
      conflationResult = ConflationCalculationResult(
        startBlockNumber = start,
        endBlockNumber = end,
        conflationTrigger = ConflationTrigger.BLOCKS_LIMIT,
        tracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
      ),
    )
  }

  @Test
  fun `handleConflatedBatch throws when conflation arrives out of order`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()

    assertThatThrownBy {
      coordinator.handleConflatedBatch(makeConflation(5UL, 7UL))
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("Conflation out of order")
      .hasMessageContaining("expected startBlockNumber=4")
  }

  @Test
  fun `action does not submit proof when fewer than conflationsPerRollupProof conflations accumulated`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()

    coordinator.action().get()

    verify(batchesRepository, never()).findHighestConsecutiveEndBlockNumberFromBlockNumber(any())
    verify(rollupProverClient, never()).createProofRequest(any())
  }

  @Test
  fun `action does not submit proof when not all conflations are proven`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    coordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()

    whenever(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(1L))
      .thenReturn(SafeFuture.completedFuture(null))

    coordinator.action().get()

    verify(rollupProverClient, never()).createProofRequest(any())
  }

  @Test
  fun `action does not submit proof when only first conflation is proven`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    coordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()

    whenever(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(1L))
      .thenReturn(SafeFuture.completedFuture(3L))

    coordinator.action().get()

    verify(rollupProverClient, never()).createProofRequest(any())
  }

  @Test
  fun `action submits proof request when all N conflations are proven`() {
    val conflation1 = makeConflation(1UL, 3UL)
    val conflation2 = makeConflation(4UL, 6UL)
    val proofHash1 = ByteArray(32) { 0x11.toByte() }
    val proofHash2 = ByteArray(32) { 0x22.toByte() }

    coordinator.handleConflatedBatch(conflation1).get()
    coordinator.handleConflatedBatch(conflation2).get()

    whenever(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(1L))
      .thenReturn(SafeFuture.completedFuture(6L))
    whenever(batchesRepository.findBatchesByBlockRange(1L, 6L))
      .thenReturn(
        SafeFuture.completedFuture(
          listOf(
            Batch(1UL, 3UL, proofHash1),
            Batch(4UL, 6UL, proofHash2),
          ),
        ),
      )

    val returnedProofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1UL,
      endBlockNumber = 6UL,
      startBlockTimestamp = Instant.fromEpochSeconds(12),
      hash = ByteArray(32) { 0xFF.toByte() },
    )
    whenever(rollupProverClient.createProofRequest(any()))
      .thenReturn(SafeFuture.completedFuture(returnedProofIndex))

    coordinator.action().get()

    val requestCaptor = argumentCaptor<linea.clients.RollupProofRequestV1>()
    verify(rollupProverClient).createProofRequest(requestCaptor.capture())

    val request = requestCaptor.firstValue
    assertThat(request.startBlockNumber).isEqualTo(1UL)
    assertThat(request.endBlockNumber).isEqualTo(6UL)
    assertThat(request.startOffset).isEqualTo(0)
    assertThat(request.chunks).hasSize(1)
    assertThat(request.chunks.first()).isEqualTo(fakeChunkHash)
    assertThat(request.parentDataRollingHash).isEqualTo(genesisDataRollingHash)
    assertThat(request.opaquePrefixBytes).isEmpty()
    // partial chunk has segmentSize * 2 bytes used; suffix pads to BLOB_SIZE
    assertThat(request.opaqueSuffixBytes).hasSize(Constants.Eip4844BlobSize - segmentSize * conflationsPerRollupProof)
    assertThat(request.l2Executions).hasSize(2)
    assertThat(request.l2Executions[0].startBlockNumber).isEqualTo(1UL)
    assertThat(request.l2Executions[0].endBlockNumber).isEqualTo(3UL)
    assertThat(request.l2Executions[0].hash).isEqualTo(proofHash1)
    assertThat(request.l2Executions[1].startBlockNumber).isEqualTo(4UL)
    assertThat(request.l2Executions[1].endBlockNumber).isEqualTo(6UL)
    assertThat(request.l2Executions[1].hash).isEqualTo(proofHash2)
  }

  @Test
  fun `action passes correct context to proof poller after submission`() {
    val conflation1 = makeConflation(1UL, 3UL)
    val conflation2 = makeConflation(4UL, 6UL)
    val proofHash1 = ByteArray(32) { 0x11.toByte() }
    val proofHash2 = ByteArray(32) { 0x22.toByte() }

    coordinator.handleConflatedBatch(conflation1).get()
    coordinator.handleConflatedBatch(conflation2).get()

    whenever(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(1L))
      .thenReturn(SafeFuture.completedFuture(6L))
    whenever(batchesRepository.findBatchesByBlockRange(1L, 6L))
      .thenReturn(
        SafeFuture.completedFuture(
          listOf(
            Batch(1UL, 3UL, proofHash1),
            Batch(4UL, 6UL, proofHash2),
          ),
        ),
      )

    val returnedProofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1UL,
      endBlockNumber = 6UL,
      startBlockTimestamp = Instant.fromEpochSeconds(12),
      hash = ByteArray(32) { 0xFF.toByte() },
    )
    whenever(rollupProverClient.createProofRequest(any()))
      .thenReturn(SafeFuture.completedFuture(returnedProofIndex))

    coordinator.action().get()

    val proofIndexCaptor = argumentCaptor<BlockIntervalProofIndex>()
    val parentDrhCaptor = argumentCaptor<ByteArray>()
    val dataRollingHashCaptor = argumentCaptor<ByteArray>()
    val endOffsetCaptor = argumentCaptor<Int>()
    val startTimestampCaptor = argumentCaptor<Instant>()
    val endTimestampCaptor = argumentCaptor<Instant>()
    val totalBatchesCaptor = argumentCaptor<Int>()
    verify(rollupProofPoller).addProofInProgress(
      proofIndex = proofIndexCaptor.capture(),
      blobsData = any(),
      parentDataRollingHash = parentDrhCaptor.capture(),
      dataRollingHash = dataRollingHashCaptor.capture(),
      endOffset = endOffsetCaptor.capture(),
      startBlockTimestamp = startTimestampCaptor.capture(),
      endBlockTimestamp = endTimestampCaptor.capture(),
      totalBatchesCount = totalBatchesCaptor.capture(),
    )
    assertThat(proofIndexCaptor.firstValue).isEqualTo(returnedProofIndex)
    assertThat(parentDrhCaptor.firstValue).isEqualTo(genesisDataRollingHash)
    // fold(genesisDataRollingHash, fakeChunkHash) = fakeChunkHash.copyOf(32) per our calculator
    assertThat(dataRollingHashCaptor.firstValue).isEqualTo(fakeChunkHash.copyOf(32))
    assertThat(endOffsetCaptor.firstValue).isEqualTo(segmentSize * conflationsPerRollupProof)
    assertThat(startTimestampCaptor.firstValue).isEqualTo(Instant.fromEpochSeconds(12))
    assertThat(endTimestampCaptor.firstValue).isEqualTo(Instant.fromEpochSeconds(6L * 12))
    assertThat(totalBatchesCaptor.firstValue).isEqualTo(conflationsPerRollupProof)
  }

  @Test
  fun `action resets window state after successful submission`() {
    val proofHash1 = ByteArray(32) { 0x11.toByte() }
    val proofHash2 = ByteArray(32) { 0x22.toByte() }

    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    coordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()
    // Third conflation queued — should remain after window reset
    coordinator.handleConflatedBatch(makeConflation(7UL, 9UL)).get()

    whenever(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(1L))
      .thenReturn(SafeFuture.completedFuture(6L))
    whenever(batchesRepository.findBatchesByBlockRange(1L, 6L))
      .thenReturn(
        SafeFuture.completedFuture(
          listOf(
            Batch(1UL, 3UL, proofHash1),
            Batch(4UL, 6UL, proofHash2),
          ),
        ),
      )

    val returnedProofIndex = BlockIntervalProofIndex(
      startBlockNumber = 1UL,
      endBlockNumber = 6UL,
      startBlockTimestamp = Instant.fromEpochSeconds(12),
      hash = ByteArray(32) { 0xFF.toByte() },
    )
    whenever(rollupProverClient.createProofRequest(any()))
      .thenReturn(SafeFuture.completedFuture(returnedProofIndex))

    coordinator.action().get()

    // Second action tick: only one conflation left — should not submit again
    whenever(batchesRepository.findHighestConsecutiveEndBlockNumberFromBlockNumber(any()))
      .thenReturn(SafeFuture.completedFuture(null))

    coordinator.action().get()

    // createProofRequest should have been called exactly once (first window only)
    verify(rollupProverClient, org.mockito.kotlin.times(1)).createProofRequest(any())
  }
}
