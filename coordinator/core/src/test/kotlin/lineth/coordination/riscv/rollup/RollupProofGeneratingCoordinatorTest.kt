package lineth.coordination.riscv.rollup

import io.vertx.core.Vertx
import linea.clients.FakeRollupProverClient
import linea.clients.RollupProofResponseV1
import linea.clients.dummyRollupProofResponse
import linea.domain.Batch
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.Constants
import linea.domain.DataRollingHashCalculator
import linea.domain.StreamPosition
import linea.domain.createBlock
import lineth.conflation.calculators.GlobalRollupCalculator
import lineth.conflation.calculators.RollupTriggerCalculatorByConflationCount
import lineth.coordination.riscv.conflation.ConflationSegmentBuilder
import lineth.encoding.BlockEncoder
import lineth.persistence.FakeBatchesRepository
import lineth.vertx.vertxTestOptions
import net.consensys.linea.traces.TracesCountersV2
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class RollupProofGeneratingCoordinatorTest {

  private val chainId = 1UL
  private val conflationsPerRollupProof = 2
  private val genesisDataRollingHash = ByteArray(32) { 0 }
  private val segmentSize = 100
  private val fakeSegment = ByteArray(segmentSize) { it.toByte() }
  private val fakeEncodedBlock = ByteArray(10) { 0xAB.toByte() }
  private val fakeChunkHash = ByteArray(32) { 0x42.toByte() }
  private val proofHash1 = ByteArray(32) { 0x11.toByte() }
  private val proofHash2 = ByteArray(32) { 0x22.toByte() }

  private val rollupProverClient = FakeRollupProverClient()
  private val batchesRepository = FakeBatchesRepository()
  private val handledProofs = CopyOnWriteArrayList<Pair<RollupProofResponseV1, RollupProofPoller.ProofContext>>()
  private val blockEncoder = BlockEncoder { fakeEncodedBlock }

  private val conflationSegmentBuilder = ConflationSegmentBuilder { _, _ -> fakeSegment }
  private val dataRollingHashCalculator = DataRollingHashCalculator { _, chunk -> chunk.copyOf(32) }
  private val chunkHasher: (ByteArray) -> ByteArray = { _ -> fakeChunkHash }

  private lateinit var vertx: Vertx
  private lateinit var rollupProofPoller: RollupProofPoller
  private lateinit var coordinator: RollupProofGeneratingCoordinator

  @BeforeEach
  fun setUp() {
    this.vertx = Vertx.vertx(vertxTestOptions)
    rollupProofPoller = RollupProofPoller(
      rollupProverClient = rollupProverClient,
      rollupProofHandler = { proof, context ->
        handledProofs.add(proof to context)
        SafeFuture.completedFuture(Unit)
      },
      vertx = vertx,
      config = RollupProofPoller.Config(pollingInterval = 1.seconds),
      metricsFacade = mock(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    )
    coordinator = createCoordinator(conflationSegmentBuilder)
  }

  private fun createCoordinator(segmentBuilder: ConflationSegmentBuilder): RollupProofGeneratingCoordinator {
    return RollupProofGeneratingCoordinator(
      chainId = chainId,
      rollupCalculator = GlobalRollupCalculator(
        listOf(RollupTriggerCalculatorByConflationCount(conflationsPerRollupProof)),
      ),
      rollupProverClient = rollupProverClient,
      batchesRepository = batchesRepository,
      streamPositionProvider = { SafeFuture.completedFuture(StreamPosition(0UL, genesisDataRollingHash)) },
      rollupProofPoller = rollupProofPoller,
      conflationSegmentBuilder = segmentBuilder,
      dataRollingHashCalculator = dataRollingHashCalculator,
      blockEncoder = blockEncoder,
      chunkHasher = chunkHasher,
      proofCheckingInterval = 1.seconds,
      vertx = vertx,
      metricsFacade = mock(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    ).also { it.start().get() }
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

  private fun markProven(start: ULong, end: ULong, proofHash: ByteArray = ByteArray(32)) {
    batchesRepository.saveNewBatch(Batch(start, end, proofHash)).get()
  }

  @Test
  fun `handleConflatedBatch throws when conflation arrives out of order`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()

    assertThatThrownBy {
      coordinator.handleConflatedBatch(makeConflation(5UL, 7UL)).get()
    }.hasCauseInstanceOf(IllegalArgumentException::class.java)
      .cause()
      .hasMessageContaining("Conflation out of order")
      .hasMessageContaining("expected startBlockNumber=4")
  }

  @Test
  fun `action does not submit proof when fewer than conflationsPerRollupProof conflations accumulated`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    markProven(1UL, 3UL)

    coordinator.action().get()

    assertThat(rollupProverClient.requests).isEmpty()
  }

  @Test
  fun `action does not submit proof when not all conflations are proven`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    coordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()

    coordinator.action().get()

    assertThat(rollupProverClient.requests).isEmpty()
  }

  @Test
  fun `action does not submit proof when only first conflation is proven`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    coordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()
    markProven(1UL, 3UL)

    coordinator.action().get()

    assertThat(rollupProverClient.requests).isEmpty()
  }

  @Test
  fun `action submits proof request when all N conflations are proven`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    coordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()
    markProven(1UL, 3UL, proofHash1)
    markProven(4UL, 6UL, proofHash2)

    coordinator.action().get()

    assertThat(rollupProverClient.requests).hasSize(1)
    val request = rollupProverClient.requests.first()
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
  fun `submitted proof is handed to the proof handler with correct context once ready`() {
    rollupProverClient.responseProvider = { null }
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    coordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()
    markProven(1UL, 3UL, proofHash1)
    markProven(4UL, 6UL, proofHash2)

    coordinator.action().get()
    rollupProofPoller.action().get()
    assertThat(handledProofs).isEmpty()

    rollupProverClient.responseProvider = ::dummyRollupProofResponse
    rollupProofPoller.action().get()

    assertThat(handledProofs).hasSize(1)
    val (proof, context) = handledProofs.first()
    val expectedProofIndex = rollupProverClient.getProofIndex(rollupProverClient.requests.single())
    assertThat(proof.startBlockNumber).isEqualTo(1UL)
    assertThat(proof.endBlockNumber).isEqualTo(6UL)
    assertThat(context.proofIndex).isEqualTo(expectedProofIndex)
    assertThat(context.parentDataRollingHash).isEqualTo(genesisDataRollingHash)
    // fold(genesisDataRollingHash, fakeChunkHash) = fakeChunkHash.copyOf(32) per our calculator
    assertThat(context.dataRollingHash).isEqualTo(fakeChunkHash.copyOf(32))
    assertThat(context.endOffset).isEqualTo(segmentSize * conflationsPerRollupProof)
    assertThat(context.startBlockTimestamp).isEqualTo(Instant.fromEpochSeconds(12))
    assertThat(context.endBlockTimestamp).isEqualTo(Instant.fromEpochSeconds(6L * 12))
    assertThat(context.totalBatchesCount).isEqualTo(conflationsPerRollupProof)

    // handled proofs are no longer polled
    rollupProofPoller.action().get()
    assertThat(handledProofs).hasSize(1)
  }

  @Test
  fun `action resets window state after successful submission`() {
    coordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    coordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()
    // Third conflation queued — should remain after window reset
    coordinator.handleConflatedBatch(makeConflation(7UL, 9UL)).get()
    markProven(1UL, 3UL, proofHash1)
    markProven(4UL, 6UL, proofHash2)

    coordinator.action().get()
    // Second action tick: third conflation proven but window not full — should not submit again
    markProven(7UL, 9UL)
    coordinator.action().get()

    assertThat(rollupProverClient.requests).hasSize(1)
    assertThat(rollupProverClient.requests.first().endBlockNumber).isEqualTo(6UL)
  }

  @Test
  fun `action submits proof when segment bytes exactly fill full chunks with no remainder`() {
    // Each segment is exactly half of BLOB_SIZE so two conflations = one full chunk, no remainder
    val halfBlobSize = Constants.Eip4844BlobSize / 2
    val exactSegment = ByteArray(halfBlobSize) { it.toByte() }
    val exactCoordinator = createCoordinator { _, _ -> exactSegment }

    exactCoordinator.handleConflatedBatch(makeConflation(1UL, 3UL)).get()
    exactCoordinator.handleConflatedBatch(makeConflation(4UL, 6UL)).get()
    markProven(1UL, 3UL, proofHash1)
    markProven(4UL, 6UL, proofHash2)

    exactCoordinator.action().get()
    rollupProofPoller.action().get()

    assertThat(rollupProverClient.requests).hasSize(1)
    val request = rollupProverClient.requests.first()
    // One full chunk sealed, no partial — endOffset on the full chunk is BLOB_SIZE
    assertThat(request.chunks).hasSize(1)
    assertThat(request.opaqueSuffixBytes).isEmpty()
    assertThat(handledProofs).hasSize(1)
    assertThat(handledProofs.first().second.endOffset).isEqualTo(Constants.Eip4844BlobSize)
  }
}
