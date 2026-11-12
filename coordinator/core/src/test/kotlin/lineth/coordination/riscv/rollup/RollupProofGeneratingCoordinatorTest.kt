package lineth.coordination.riscv.rollup

import io.vertx.core.Vertx
import linea.clients.FakeRollupProverClient
import linea.clients.RollupProofRequestV1
import linea.clients.RollupProofResponseV1
import linea.clients.RollupProverClientV1
import linea.clients.dummyRollupProofResponse
import linea.da.DaCodecVersion
import linea.da.DaConstants
import linea.da.DaStreamException
import linea.da.DaStreamPosition
import linea.da.DaStreamStatus
import linea.da.RollupResumePoint
import linea.domain.Batch
import linea.domain.Block
import linea.domain.BlockIntervalProofIndex
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import lineth.conflation.calculators.BlobTriggerCalculator
import lineth.conflation.calculators.BlobTriggerCalculatorByTargetBlockNumbers
import lineth.conflation.calculators.DaTestConflations
import lineth.conflation.calculators.DaTestConflations.baseTime
import lineth.conflation.calculators.DaTestConflations.genesisDataRollingHash
import lineth.conflation.calculators.RollupTriggerCalculator
import lineth.conflation.calculators.RollupTriggerCalculatorByConflationCount
import lineth.coordination.riscv.da.FakeDaStream
import lineth.coordination.riscv.da.FakeDaStreamFactory
import lineth.persistence.FakeBatchesRepository
import lineth.vertx.vertxTestOptions
import net.consensys.FakeFixedClock
import net.consensys.linea.traces.TracesCountersV5
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

class RollupProofGeneratingCoordinatorTest {
  /** Fails the next [failures] proof request creations. */
  private class FlakyRollupProverClient(
    val delegate: FakeRollupProverClient,
  ) : RollupProverClientV1 by delegate {
    var failures = 0

    override fun createProofRequest(proofRequest: RollupProofRequestV1): SafeFuture<BlockIntervalProofIndex> {
      if (failures > 0) {
        failures--
        return SafeFuture.failedFuture(RuntimeException("prover unavailable"))
      }
      return delegate.createProofRequest(proofRequest)
    }
  }

  private val l2Blocks = mutableMapOf<ULong, Block>()

  // blob1 = [A, start of B], blob2 = [end of B, C, start of D], blob3 = [middle of D], blob4 = [end of D, E].
  // blob3 starts and ends inside D, so it has no cut.
  private val a = conflation(1UL, 50_000)
  private val b = conflation(2UL, 100_000)
  private val c = conflation(3UL, 40_000)
  private val d = conflation(4UL, 70_094 + DaConstants.BLOB_PAYLOAD_CAPACITY + 9_906)
  private val e = conflation(5UL, 30_000)

  private val batchesRepository = FakeBatchesRepository()
  private val proverClient = FlakyRollupProverClient(FakeRollupProverClient(responseProvider = { null }))
  private val requests: List<RollupProofRequestV1> get() = proverClient.delegate.requests
  private val handledProofs = CopyOnWriteArrayList<Pair<RollupProofResponseV1, RollupProofPoller.ProofContext>>()
  private val fetchedRanges = CopyOnWriteArrayList<ULongRange>()
  private val clock = FakeFixedClock(baseTime)

  private lateinit var vertx: Vertx
  private lateinit var rollupProofPoller: RollupProofPoller

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx(vertxTestOptions)
    rollupProofPoller = RollupProofPoller(
      rollupProverClient = proverClient,
      rollupProofHandler = { proof, context ->
        handledProofs.add(proof to context)
        SafeFuture.completedFuture(Unit)
      },
      vertx = vertx,
      config = RollupProofPoller.Config(pollingInterval = 1.seconds),
      metricsFacade = mock(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    )
  }

  @AfterEach
  fun tearDown() {
    vertx.close().toCompletionStage().toCompletableFuture().get()
  }

  private fun conflation(number: ULong, frameSize: Int): BlocksConflation =
    DaTestConflations.conflation(number, frameSize).also { conflation ->
      conflation.blocks.forEach { l2Blocks[it.number] = it }
    }

  private fun merged(first: BlocksConflation, second: BlocksConflation): BlocksConflation =
    BlocksConflation(
      first.blocks + second.blocks,
      ConflationCalculationResult(
        first.startBlockNumber,
        second.endBlockNumber,
        ConflationTrigger.BLOCKS_LIMIT,
        TracesCountersV5.EMPTY_TRACES_COUNT,
      ),
    )

  private fun proofHash(conflation: BlocksConflation) = ByteArray(32) { conflation.startBlockNumber.toByte() }

  private fun prove(vararg conflations: BlocksConflation) {
    conflations.forEach {
      batchesRepository.saveNewBatch(Batch(it.startBlockNumber, it.endBlockNumber, proofHash(it))).get()
    }
  }

  private fun createCoordinator(
    resumePoint: RollupResumePoint = RollupResumePoint(0UL, DaStreamPosition(genesisDataRollingHash)),
    rollupTriggers: List<RollupTriggerCalculator> = listOf(RollupTriggerCalculatorByConflationCount(3)),
    blobTriggers: List<BlobTriggerCalculator> = emptyList(),
    rollupDeadline: Duration? = null,
  ): RollupProofGeneratingCoordinator =
    RollupProofGeneratingCoordinator(
      rollupProverClient = proverClient,
      batchesRepository = batchesRepository,
      resumePointSupplier = { SafeFuture.completedFuture(resumePoint) },
      rollupProofPoller = rollupProofPoller,
      l2BlocksFetcher = { start, end ->
        fetchedRanges += start..end
        SafeFuture.completedFuture((start..end).map { l2Blocks.getValue(it) })
      },
      daStreamFactory = FakeDaStreamFactory(),
      blobTriggers = blobTriggers,
      rollupTriggers = rollupTriggers,
      blockEncoder = DaTestConflations.encoder,
      config = RollupProofGeneratingCoordinator.Config(
        pollingInterval = 1.seconds,
        daCodecVersion = DaCodecVersion.FAKE,
        dictionaryPath = Path.of("unused"),
        rollupDeadline = rollupDeadline,
      ),
      vertx = vertx,
      clock = clock,
    )

  /** Delivers the proofs of all tracked requests to the handler and returns their contexts, in request order. */
  private fun pollProofs(): List<RollupProofPoller.ProofContext> {
    proverClient.delegate.responseProvider = ::dummyRollupProofResponse
    rollupProofPoller.action().get()
    return handledProofs.map { it.second }.sortedBy { it.proofIndex.startBlockNumber }
  }

  @Test
  fun `conflations are fed only once their batch is persisted`() {
    val coordinator = createCoordinator()
    listOf(a, b, c, d).forEach { coordinator.handleConflatedBatch(it).get() }

    coordinator.action().get()
    assertThat(requests).isEmpty()

    // Rollup 1..3 ends inside blob2, which is sealed only once D is appended.
    prove(a, b, c)
    coordinator.action().get()
    assertThat(requests).isEmpty()

    prove(d)
    coordinator.action().get()
    assertThat(requests.single().startBlockNumber).isEqualTo(1UL)
    assertThat(requests.single().endBlockNumber).isEqualTo(3UL)
    assertThat(fetchedRanges).`as`("all blocks came from the cache").isEmpty()
  }

  @Test
  fun `requests carry packed blobs, and the next rollup starts inside the shared blob`() {
    val coordinator = createCoordinator(blobTriggers = listOf(BlobTriggerCalculatorByTargetBlockNumbers(setOf(5UL))))
    listOf(a, b, c, d, e).forEach { coordinator.handleConflatedBatch(it).get() }
    prove(a, b, c, d, e)

    coordinator.action().get()

    val (request1, request2) = requests
    assertThat(request1.chunks).hasSize(2).allSatisfy { assertThat(it.blobBytes).hasSize(DaConstants.BLOB_BYTES) }
    // Each chunk carries the stream's KZG commitment and proof for its blob.
    assertThat(request1.chunks).allSatisfy {
      assertThat(it.kzgCommitment).isEqualTo(FakeDaStream.fakeKzgCommitment(it.blobBytes))
      assertThat(it.kzgProof).isEqualTo(FakeDaStream.fakeKzgProof(it.blobBytes))
    }
    assertThat(request1.parentDataRollingHash).isEqualTo(genesisDataRollingHash)
    assertThat(request1.parentDataTailTakeBytes).isZero()
    assertThat(request1.boundaryPrevDataRollingHash).isNull()
    assertThat(request1.l2Executions.map { it.hash }).containsExactly(proofHash(a), proofHash(b), proofHash(c))
    assertThat(request2.startBlockNumber).isEqualTo(4UL)
    assertThat(request2.chunks).hasSize(3)
    assertThat(request2.chunks.first()).isEqualTo(request1.chunks.last())
    assertThat(request2.parentDataTailTakeBytes).isEqualTo(70_094)

    val (context1, context2) = pollProofs()
    assertThat(context1.finalDataTailDiscardBytes).isEqualTo(70_094)
    assertThat(context1.blobsData.map { it.batchesCount }).containsExactly(1U, 2U)
    assertThat(context1.totalBatchesCount).isEqualTo(3)
    assertThat(request2.parentDataRollingHash).isEqualTo(context1.dataRollingHash)
    assertThat(request2.boundaryPrevDataRollingHash).isEqualTo(context1.blobsData.last().prevDataRollingHash)
    assertThat(context2.parentDataTailTakeBytes).isEqualTo(70_094)
    assertThat(context2.blobsData.map { it.batchesCount }).containsExactly(2U, 0U, 2U)
    assertThat(context2.finalDataTailDiscardBytes).isZero()
  }

  @Test
  fun `feeding uses the persisted batch boundaries, fetching blocks when the cache does not match`() {
    val coordinator = createCoordinator(blobTriggers = listOf(BlobTriggerCalculatorByTargetBlockNumbers(setOf(2UL))))
    coordinator.handleConflatedBatch(merged(a, b)).get()
    prove(a, b)

    coordinator.action().get()

    assertThat(fetchedRanges).containsExactly(1UL..1UL, 2UL..2UL)
    assertThat(requests.single().l2Executions.map { it.startBlockNumber..it.endBlockNumber })
      .containsExactly(1UL..1UL, 2UL..2UL)
  }

  @Test
  fun `a failed proof request is retried on the next tick and not lost`() {
    val coordinator = createCoordinator(blobTriggers = listOf(BlobTriggerCalculatorByTargetBlockNumbers(setOf(1UL))))
    prove(a)
    proverClient.failures = 1

    assertThatThrownBy { coordinator.action().get() }.hasMessageContaining("prover unavailable")
    assertThat(requests).isEmpty()

    coordinator.action().get()
    coordinator.action().get()
    assertThat(requests.map { it.endBlockNumber }).containsExactly(1UL)
  }

  @Test
  fun `the rollup deadline flushes the open blob and ends the rollup`() {
    val coordinator = createCoordinator(rollupDeadline = 1.minutes)
    prove(a)

    clock.setTimeTo(baseTime)
    coordinator.action().get()
    assertThat(requests).isEmpty()

    clock.setTimeTo(baseTime + 2.minutes)
    coordinator.action().get()
    assertThat(requests.single().endBlockNumber).isEqualTo(1UL)
  }

  @Test
  fun `a restart from the persisted rollup reproduces the next request from persisted batches`() {
    val blobTriggers = { listOf(BlobTriggerCalculatorByTargetBlockNumbers(setOf(5UL))) }
    val uninterrupted = createCoordinator(blobTriggers = blobTriggers())
    prove(a, b, c, d, e)
    uninterrupted.action().get()
    val (_, expectedRequest2) = requests
    val context1 = pollProofs().first()
    proverClient.delegate.requests.clear()
    proverClient.delegate.responseProvider = { null }
    fetchedRanges.clear()

    val restarted = createCoordinator(resumePoint = resumePointAfter(context1), blobTriggers = blobTriggers())
    restarted.action().get()

    assertThat(fetchedRanges).containsExactly(4UL..4UL, 5UL..5UL)
    assertThat(requests.single()).isEqualTo(expectedRequest2)
  }

  @Test
  fun `a rollup whose proof already exists is tracked without a new request`() {
    val coordinator = createCoordinator(blobTriggers = listOf(BlobTriggerCalculatorByTargetBlockNumbers(setOf(1UL))))
    prove(a)
    proverClient.delegate.responseProvider = ::dummyRollupProofResponse

    coordinator.action().get()
    rollupProofPoller.action().get()

    assertThat(requests).isEmpty()
    assertThat(handledProofs.map { it.second.proofIndex.endBlockNumber }).containsExactly(1UL)
  }

  @Test
  fun `a persisted batch that does not reproduce the shared blob stops the rollup stage`() {
    val uninterrupted = createCoordinator()
    prove(a, b, c, d)
    uninterrupted.action().get()
    val context1 = pollProofs().first()

    // After the restart, blocks 4..5 are persisted as one batch: its frame differs from D's.
    batchesRepository.deleteBatchesAfterBlockNumber(4L).get()
    prove(merged(d, e))
    val restarted = createCoordinator(resumePoint = resumePointAfter(context1))

    val error = runCatching { restarted.action().get() }.exceptionOrNull()
    assertThat((error?.cause as? DaStreamException)?.status).isEqualTo(DaStreamStatus.SEEDED_BLOB_MISMATCH)
    fetchedRanges.clear()
    restarted.action().get()
    assertThat(fetchedRanges).`as`("the stopped stage reads nothing more").isEmpty()
  }

  /** The resume point the persisted row of [context] gives, as BlobsRepositoryV2RollupResumePointProvider builds it. */
  private fun resumePointAfter(context: RollupProofPoller.ProofContext): RollupResumePoint {
    val tail = context.finalDataTailDiscardBytes
    val lastBlob = context.blobsData.last()
    return RollupResumePoint(
      context.proofIndex.endBlockNumber,
      DaStreamPosition(
        dataRollingHash = context.dataRollingHash,
        tailBytes = tail,
        sharedBlob = lastBlob.blobBytes.takeIf { tail > 0 },
        prevDataRollingHash = lastBlob.prevDataRollingHash.takeIf { tail > 0 },
      ),
    )
  }
}
