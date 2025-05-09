package net.consensys.zkevm.ethereum.submission

import io.vertx.core.Vertx
import linea.domain.BlockIntervals
import linea.domain.toBlockIntervals
import net.consensys.FakeFixedClock
import net.consensys.linea.async.AsyncFilter
import net.consensys.zkevm.coordinator.clients.smartcontract.BlockAndNonce
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.createAggregation
import net.consensys.zkevm.domain.createBlobRecords
import net.consensys.zkevm.persistence.AggregationsRepository
import net.consensys.zkevm.persistence.BlobsRepository
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.atLeast
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.spy
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class BlobSubmissionCoordinatorTest {
  private lateinit var lineaRollupSmartContractClient: LineaRollupSmartContractClient
  private lateinit var blobsRepository: BlobsRepository
  private lateinit var aggregationsRepository: AggregationsRepository
  private lateinit var blobSubmitter: BlobSubmitter
  private lateinit var blobSubmissionFilter: AsyncFilter<BlobRecord>
  private lateinit var blobsGrouperForSubmission: BlobsGrouperForSubmission
  private lateinit var vertx: Vertx
  private lateinit var fakeClock: FakeFixedClock
  private lateinit var blobSubmissionCoordinator: BlobSubmissionCoordinator
  private lateinit var log: Logger
  private val blobs = createBlobRecords(
    blobsIntervals = BlockIntervals(
      startingBlockNumber = 1u,
      upperBoundaries = listOf(
        // Agg1
        19u, 29u, 39u, 49u, 59u, 69u,
        79u, 89u, 99u, 109u, 119u, 129u,
        139u, 149u, 159u,
        // Agg2
        169u, 179u, 189u, 199u
      )
    )
  )
  private val aggregations = listOf(
    createAggregation(startBlockNumber = 1, endBlockNumber = 159),
    createAggregation(startBlockNumber = 160, endBlockNumber = 199)
  )

  @BeforeEach
  fun beforeEach() {
    blobsRepository = mock()
    aggregationsRepository = mock()
    lineaRollupSmartContractClient = mock() {
      on { updateNonceAndReferenceBlockToLastL1Block() }
        .thenReturn(SafeFuture.completedFuture(BlockAndNonce(blockNumber = 1u, nonce = 1u)))
      on { finalizedL2BlockNumber() }.thenReturn(SafeFuture.completedFuture(0u))
    }
    blobSubmitter = mock()
    blobSubmissionFilter = spy(AsyncFilter.NoOp())
    blobsGrouperForSubmission = spy(object : BlobsGrouperForSubmission {
      override fun chunkBlobs(
        blobsIntervals: List<BlobRecord>,
        aggregations: BlockIntervals
      ): List<List<BlobRecord>> = chunkBlobs(blobsIntervals, aggregations, targetChunkSize = 6)
    })

    vertx = Vertx.vertx()
    fakeClock = FakeFixedClock()
    log = spy(LogManager.getLogger(BlobSubmissionCoordinator::class.java))

    blobSubmissionCoordinator = spy(
      BlobSubmissionCoordinator(
        config = BlobSubmissionCoordinator.Config(
          pollingInterval = 100.milliseconds,
          proofSubmissionDelay = 0.seconds,
          maxBlobsToSubmitPerTick = 200u,
          targetBlobsToSubmitPerTx = 9u
        ),
        blobsRepository = blobsRepository,
        aggregationsRepository = aggregationsRepository,
        lineaRollup = lineaRollupSmartContractClient,
        blobSubmitter = blobSubmitter,
        vertx = vertx,
        clock = fakeClock,
        blobSubmissionFilter = blobSubmissionFilter,
        blobsGrouperForSubmission = blobsGrouperForSubmission,
        log = log
      )
    )

    whenever(blobsRepository.getConsecutiveBlobsFromBlockNumber(any(), any()))
      .thenReturn(SafeFuture.completedFuture(blobs))
    whenever(aggregationsRepository.getProofsToFinalize(any(), any(), any()))
      .thenReturn(SafeFuture.completedFuture(aggregations.map { it.aggregationProof!! }))
  }

  @Test
  fun `when eth_call fails should not submit the tx, log cause and not bubble up the exception`() {
    whenever(blobSubmitter.submitBlobCall(any()))
      .thenReturn(SafeFuture.failedFuture<Unit>(RuntimeException("eth_call failed")))

    blobSubmissionCoordinator.action()

    verify(blobSubmitter).submitBlobCall(any())
    verify(blobSubmitter, never()).submitBlobs(any())
    verify(blobSubmissionCoordinator, never()).handleError(any())
  }

  @Test
  fun `when eth_call is success should submit blobs`() {
    whenever(blobSubmitter.submitBlobCall(any()))
      .thenReturn(SafeFuture.completedFuture("txHash"))

    val blobChunks = blobsGrouperForSubmission.chunkBlobs(blobs, aggregations.toBlockIntervals())

    blobSubmissionCoordinator.action()

    verify(blobSubmitter, atLeast(1)).submitBlobCall(eq(blobChunks.first()))
    verify(blobSubmitter).submitBlobs(blobChunks)
  }
}
