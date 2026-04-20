package net.consensys.zkevm.ethereum.coordination.blob

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.clients.BlobCompressionProverClientV2
import linea.domain.Blob
import linea.domain.BlobCompressionProof
import linea.domain.BlobCompressionProofRequest
import linea.domain.BlockIntervals
import linea.domain.CompressionProofIndex
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import linea.domain.ShnarfResult
import net.consensys.FakeFixedClock
import net.consensys.linea.traces.TracesCountersV2
import org.awaitility.Awaitility
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.TestInstance
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
@TestInstance(TestInstance.Lifecycle.PER_CLASS)
class BlobCompressionProofCoordinatorTest {
  private lateinit var blobCompressionProofCoordinator: BlobCompressionProofCoordinator
  private val rollingBlobShnarfCalculator = mock<RollingBlobShnarfCalculator>()

  private val fixedClock = FakeFixedClock()
  private val blobHandlerPollingInterval = 50.milliseconds

  private val expectedStartBlock = 1UL
  private val expectedEndBlock = 100UL
  private val blobCompressionProverClient = mock<BlobCompressionProverClientV2>().also {
    val expectedBlobCompressionProofResponse = BlobCompressionProof(
      compressedData = Random.Default.nextBytes(32),
      conflationOrder = BlockIntervals(
        startingBlockNumber =
        expectedStartBlock,
        upperBoundaries =
        listOf(expectedEndBlock),
      ),
      prevShnarf = Random.Default.nextBytes(32),
      parentStateRootHash = Random.Default.nextBytes(32),
      finalStateRootHash = Random.Default.nextBytes(32),
      parentDataHash = Random.Default.nextBytes(32),
      dataHash = Random.Default.nextBytes(32),
      snarkHash = Random.Default.nextBytes(32),
      expectedX = Random.Default.nextBytes(32),
      expectedY = Random.Default.nextBytes(32),
      expectedShnarf = Random.Default.nextBytes(32),
      decompressionProof = Random.Default.nextBytes(512),
      proverVersion = "mock-0.0.0",
      verifierID = 6789,
      commitment = Random.Default.nextBytes(48),
      kzgProofContract = Random.Default.nextBytes(48),
      kzgProofSidecar = Random.Default.nextBytes(48),
    )

    whenever(it.createProofRequest(any()))
      .thenAnswer { invocationOnMock ->
        val request = invocationOnMock.getArgument<BlobCompressionProofRequest>(0)
        SafeFuture.completedFuture(
          CompressionProofIndex(
            startBlockNumber = request.conflations.first().startBlockNumber,
            endBlockNumber = request.conflations.last().endBlockNumber,
            hash = request.expectedShnarfResult.expectedShnarf,
            startBlockTimestamp = request.startBlockTimestamp,
          ),
        )
      }
    whenever(it.findProofResponse(any()))
      .thenReturn(SafeFuture.completedFuture(expectedBlobCompressionProofResponse))
  }
  private val blobZkStateProvider = mock<BlobZkStateProvider>()

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    blobCompressionProofCoordinator = BlobCompressionProofCoordinator(
      vertx = vertx,
      blobCompressionProverClient = blobCompressionProverClient,
      rollingBlobShnarfCalculator = rollingBlobShnarfCalculator,
      blobZkStateProvider = blobZkStateProvider,
      config = BlobCompressionProofCoordinator.Config(
        pollingInterval = blobHandlerPollingInterval,
      ),
      blobCompressionProofHandler = { _ -> SafeFuture.completedFuture(Unit) },
      metricsFacade = mock(defaultAnswer = Mockito.RETURNS_DEEP_STUBS),
    )
    blobCompressionProofCoordinator.start()
  }

  @Test
  fun `guarantee requestBlobCompressionProof is being passed the correct values in the correct order`() {
    val startBlockTime = fixedClock.now()
    val expectedParentDataHash = Random.Default.nextBytes(32)
    val expectedPrevShnarf = Random.Default.nextBytes(32)
    val parentStateRootHash = Random.Default.nextBytes(32)
    val finalStateRootHash = Random.Default.nextBytes(32)

    val blob = Blob(
      conflations = listOf(
        ConflationCalculationResult(
          startBlockNumber = expectedStartBlock,
          endBlockNumber = expectedEndBlock,
          conflationTrigger = ConflationTrigger.TRACES_LIMIT,
          tracesCounters = TracesCountersV2.Companion.EMPTY_TRACES_COUNT,
        ),
      ),
      compressedData = Random.Default.nextBytes(128),
      startBlockTime = startBlockTime,
      endBlockTime = fixedClock.now().plus((12 * (expectedEndBlock - expectedStartBlock).toInt()).seconds),
    )

    whenever(blobZkStateProvider.getBlobZKState(any()))
      .thenReturn(
        SafeFuture.completedFuture(
          BlobZkState(
            parentStateRootHash = parentStateRootHash,
            finalStateRootHash = finalStateRootHash,
          ),
        ),
      )

    val shnarfResult = ShnarfResult(
      dataHash = Random.Default.nextBytes(32),
      snarkHash = Random.Default.nextBytes(32),
      expectedX = Random.Default.nextBytes(32),
      expectedY = Random.Default.nextBytes(32),
      expectedShnarf = Random.Default.nextBytes(32),
      commitment = Random.Default.nextBytes(48),
      kzgProofContract = Random.Default.nextBytes(48),
      kzgProofSideCar = Random.Default.nextBytes(48),
    )

    whenever(rollingBlobShnarfCalculator.calculateShnarf(any(), any(), any(), any()))
      .thenAnswer {
        SafeFuture.completedFuture(
          RollingBlobShnarfResult(
            shnarfResult = shnarfResult,
            parentBlobHash = expectedParentDataHash,
            parentBlobShnarf = expectedPrevShnarf,
          ),
        )
      }

    blobCompressionProofCoordinator.handleBlob(blob).get()

    Awaitility.await()
      .untilAsserted {
        verify(blobCompressionProverClient)
          .createProofRequest(
            BlobCompressionProofRequest(
              compressedData = blob.compressedData,
              conflations = blob.conflations,
              parentStateRootHash = parentStateRootHash,
              finalStateRootHash = finalStateRootHash,
              parentDataHash = expectedParentDataHash,
              prevShnarf = expectedPrevShnarf,
              expectedShnarfResult = shnarfResult,
              commitment = shnarfResult.commitment,
              kzgProofContract = shnarfResult.kzgProofContract,
              kzgProofSideCar = shnarfResult.kzgProofSideCar,
              startBlockTimestamp = startBlockTime,
            ),
          )
        verify(blobCompressionProverClient, times(1))
          .findProofResponse(
            CompressionProofIndex(
              startBlockNumber = expectedStartBlock,
              endBlockNumber = expectedEndBlock,
              hash = shnarfResult.expectedShnarf,
              startBlockTimestamp = startBlockTime,
            ),
          )
      }
  }

  @Test
  fun `verify failed blob is re-queued and processed in order`() {
    val startBlockTime = fixedClock.now()
    val expectedParentDataHash = Random.Default.nextBytes(32)
    val expectedPrevShnarf = Random.Default.nextBytes(32)
    val parentStateRootHash = Random.Default.nextBytes(32)
    val finalStateRootHash = Random.Default.nextBytes(32)

    val blob1 = Blob(
      conflations = listOf(
        ConflationCalculationResult(
          startBlockNumber = 1uL,
          endBlockNumber = 10uL,
          conflationTrigger = ConflationTrigger.TRACES_LIMIT,
          tracesCounters = TracesCountersV2.Companion.EMPTY_TRACES_COUNT,
        ),
      ),
      compressedData = Random.Default.nextBytes(128),
      startBlockTime = startBlockTime,
      endBlockTime = fixedClock.now().plus((12 * (10 - 1)).seconds),
    )

    val blob2 = Blob(
      conflations = listOf(
        ConflationCalculationResult(
          startBlockNumber = 11uL,
          endBlockNumber = 20uL,
          conflationTrigger = ConflationTrigger.TRACES_LIMIT,
          tracesCounters = TracesCountersV2.Companion.EMPTY_TRACES_COUNT,
        ),
      ),
      compressedData = Random.Default.nextBytes(128),
      startBlockTime = startBlockTime,
      endBlockTime = fixedClock.now().plus((12 * (20 - 11)).seconds),
    )

    whenever(blobZkStateProvider.getBlobZKState(any()))
      .thenReturn(
        SafeFuture.failedFuture(RuntimeException("Forced blobZkStateProvider mock failure")),
        SafeFuture.completedFuture(
          BlobZkState(
            parentStateRootHash = parentStateRootHash,
            finalStateRootHash = finalStateRootHash,
          ),
        ),
        SafeFuture.completedFuture(
          BlobZkState(
            parentStateRootHash = parentStateRootHash,
            finalStateRootHash = finalStateRootHash,
          ),
        ),
      )

    val shnarfResult = ShnarfResult(
      dataHash = Random.Default.nextBytes(32),
      snarkHash = Random.Default.nextBytes(32),
      expectedX = Random.Default.nextBytes(32),
      expectedY = Random.Default.nextBytes(32),
      expectedShnarf = Random.Default.nextBytes(32),
      commitment = Random.Default.nextBytes(48),
      kzgProofContract = Random.Default.nextBytes(48),
      kzgProofSideCar = Random.Default.nextBytes(48),
    )

    whenever(rollingBlobShnarfCalculator.calculateShnarf(any(), any(), any(), any()))
      .thenAnswer {
        SafeFuture.completedFuture(
          RollingBlobShnarfResult(
            shnarfResult = shnarfResult,
            parentBlobHash = expectedParentDataHash,
            parentBlobShnarf = expectedPrevShnarf,
          ),
        )
      }

    blobCompressionProofCoordinator.handleBlob(blob1).get()
    blobCompressionProofCoordinator.handleBlob(blob2).get()

    Awaitility.await()
      .untilAsserted {
        verify(blobCompressionProverClient, times(1))
          .createProofRequest(
            BlobCompressionProofRequest(
              compressedData = blob1.compressedData,
              conflations = blob1.conflations,
              parentStateRootHash = parentStateRootHash,
              finalStateRootHash = finalStateRootHash,
              parentDataHash = expectedParentDataHash,
              prevShnarf = expectedPrevShnarf,
              expectedShnarfResult = shnarfResult,
              commitment = shnarfResult.commitment,
              kzgProofContract = shnarfResult.kzgProofContract,
              kzgProofSideCar = shnarfResult.kzgProofSideCar,
              startBlockTimestamp = blob1.startBlockTime,
            ),
          )
        verify(blobCompressionProverClient, times(1))
          .createProofRequest(
            BlobCompressionProofRequest(
              compressedData = blob2.compressedData,
              conflations = blob2.conflations,
              parentStateRootHash = parentStateRootHash,
              finalStateRootHash = finalStateRootHash,
              parentDataHash = expectedParentDataHash,
              prevShnarf = expectedPrevShnarf,
              expectedShnarfResult = shnarfResult,
              commitment = shnarfResult.commitment,
              kzgProofContract = shnarfResult.kzgProofContract,
              kzgProofSideCar = shnarfResult.kzgProofSideCar,
              startBlockTimestamp = blob2.startBlockTime,
            ),
          )

        verify(blobCompressionProverClient, times(1))
          .findProofResponse(
            CompressionProofIndex(
              startBlockNumber = blob1.startBlockNumber,
              endBlockNumber = blob1.endBlockNumber,
              hash = shnarfResult.expectedShnarf,
              startBlockTimestamp = blob1.startBlockTime,
            ),
          )
        verify(blobCompressionProverClient, times(1))
          .findProofResponse(
            CompressionProofIndex(
              startBlockNumber = blob2.startBlockNumber,
              endBlockNumber = blob2.endBlockNumber,
              hash = shnarfResult.expectedShnarf,
              startBlockTimestamp = blob2.startBlockTime,
            ),
          )
      }
  }
}
