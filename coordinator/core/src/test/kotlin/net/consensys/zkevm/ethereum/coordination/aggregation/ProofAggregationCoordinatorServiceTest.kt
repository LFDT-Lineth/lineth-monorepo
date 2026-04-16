package net.consensys.zkevm.ethereum.coordination.aggregation

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.BlockIntervals
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.zkevm.coordinator.clients.ProofAggregationProverClientV2
import net.consensys.zkevm.domain.Aggregation
import net.consensys.zkevm.domain.AggregationProofIndex
import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.domain.InvalidityProofIndex
import net.consensys.zkevm.domain.ProofsToAggregate
import net.consensys.zkevm.domain.createProofToFinalize
import net.consensys.zkevm.persistence.AggregationsRepository
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito.anyLong
import org.mockito.kotlin.any
import org.mockito.kotlin.argThat
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicInteger
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class ProofAggregationCoordinatorServiceTest {
  private val blobsToPoll = 500U

  private fun createBlob(startBlockNumber: ULong, endBLockNumber: ULong): BlobAndBatchCounters {
    val batches =
      BlockIntervals(
        startBlockNumber,
        listOf(startBlockNumber + 2UL, startBlockNumber + 6UL, endBLockNumber),
      )

    val blobCounters =
      BlobCounters(
        3u,
        startBlockNumber,
        endBLockNumber,
        Instant.fromEpochMilliseconds(100),
        Instant.fromEpochMilliseconds(5000),
        expectedShnarf = Random.nextBytes(32),
      )
    return BlobAndBatchCounters(blobCounters = blobCounters, executionProofs = batches)
  }

  private val aggregationProofResponse = createProofToFinalize(
    firstBlockNumber = 11,
    finalBlockNumber = 23,
  )

  @Test
  fun `test aggregation flow`(vertx: Vertx) {
    // FIXME this it's only happy path, with should cover other scenarios
    val mockAggregationCalculator = mock<AggregationCalculator>()
    val mockAggregationsRepository = mock<AggregationsRepository>()
    val mockProofAggregationClient = mock<ProofAggregationProverClientV2>()
    val mockAggregationL2StateProvider = mock<AggregationL2StateProvider>()
    val mockInvalidityProofProvider = mock<InvalidityProofProvider>()
    val meterRegistry = SimpleMeterRegistry()
    val metricsFacade: MetricsFacade = MicrometerMetricsFacade(registry = meterRegistry)

    val config =
      ProofAggregationCoordinatorService.Config(
        pollingInterval = 10.milliseconds,
        proofsLimit = blobsToPoll,
        proofGenerationRetryBackoffDelay = 5.milliseconds,
      )

    var provenAggregation = 0UL
    val aggregationProofRequestCaptures = mutableListOf<Pair<AggregationProofIndex, Aggregation>>()

    val proofAggregationCoordinatorService =
      ProofAggregationCoordinatorService(
        vertx = vertx,
        config = config,
        nextBlockNumberToPoll = 10L,
        aggregationCalculator = mockAggregationCalculator,
        aggregationProofHandler = { aggregation ->
          provenAggregation = aggregation.endBlockNumber
          mockAggregationsRepository.saveNewAggregation(aggregation)
        },
        aggregationProofRequestHandler = { proofIndex, unProvenAggregation ->
          aggregationProofRequestCaptures.add(proofIndex to unProvenAggregation)
        },
        consecutiveProvenBlobsProvider = mockAggregationsRepository::findConsecutiveProvenBlobs,
        proofAggregationClient = mockProofAggregationClient,
        aggregationL2StateProvider = mockAggregationL2StateProvider,
        metricsFacade = metricsFacade,
        invalidityProofProvider = mockInvalidityProofProvider,
      )
    proofAggregationCoordinatorService.aggregationProofPoller.start()

    verify(mockAggregationCalculator).onAggregation(proofAggregationCoordinatorService)

    val blob1 = listOf(createBlob(11u, 19u), createBlob(20u, 33u), createBlob(34u, 41u))
    val blob2 = listOf(createBlob(42u, 60u))
    whenever(mockAggregationsRepository.findConsecutiveProvenBlobs(anyLong()))
      .thenAnswer {
        if (it.getArgument<Long>(0) == 10L) {
          SafeFuture.completedFuture(blob1)
        } else if (it.getArgument<Long>(0) == 42L) {
          SafeFuture.completedFuture(blob2)
        } else {
          throw IllegalStateException()
        }
      }

    val blobsToAggregate1 = BlobsToAggregate(11u, 33u)
    val blobsToAggregate2 = BlobsToAggregate(34u, 60u)

    whenever(mockAggregationCalculator.newBlob(any<BlobCounters>())).thenAnswer {
      if (it.getArgument<BlobCounters>(0) == blob1[1].blobCounters) {
        proofAggregationCoordinatorService.onAggregation(blobsToAggregate1)
      } else if (it.getArgument<BlobCounters>(0) == blob2[0].blobCounters) {
        proofAggregationCoordinatorService.onAggregation(blobsToAggregate2)
      } else {
        SafeFuture.completedFuture(Unit)
      }
    }

    val compressionBlobs1 = listOf(blob1[0], blob1[1])
    val compressionBlobs2 = listOf(blob1[2], blob2[0])

    val executionProofs1 =
      BlockIntervals(
        blob1[0].executionProofs.startingBlockNumber,
        blob1[0].executionProofs.upperBoundaries + blob1[1].executionProofs.upperBoundaries,
      )

    val executionProofs2 =
      BlockIntervals(
        blob1[2].executionProofs.startingBlockNumber,
        blob1[2].executionProofs.upperBoundaries + blob2[0].executionProofs.upperBoundaries,
      )

    val rollingInfo1 =
      AggregationL2State(
        parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(123456),
        parentAggregationLastL1RollingHashMessageNumber = 12UL,
        parentAggregationLastL1RollingHash = ByteArray(32),
        parentAggregationLastFtxNumber = 0UL,
        parentAggregationLastFtxRollingHash = ByteArray(32),
      )
    val rollingInfo2 =
      AggregationL2State(
        parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(123458),
        parentAggregationLastL1RollingHashMessageNumber = 14UL,
        parentAggregationLastL1RollingHash = ByteArray(32),
        parentAggregationLastFtxNumber = 0UL,
        parentAggregationLastFtxRollingHash = ByteArray(32),
      )

    val exception1Count = AtomicInteger(100)
    val exception2Count = AtomicInteger(100)
    whenever(mockAggregationL2StateProvider.getAggregationL2State(anyLong()))
      .thenAnswer { invocation ->
        val blockNumber = invocation.getArgument<Long>(0)
        when (blockNumber) {
          blobsToAggregate1.startBlockNumber.toLong() - 1 -> {
            if (exception1Count.getAndDecrement() > 0) {
              throw IllegalArgumentException("mock exception 1")
            } else {
              SafeFuture.completedFuture(rollingInfo1)
            }
          }
          blobsToAggregate2.startBlockNumber.toLong() - 1 -> {
            if (exception2Count.getAndDecrement() > 0) {
              SafeFuture.failedFuture(IllegalArgumentException("mock exception 2"))
            } else {
              SafeFuture.completedFuture(rollingInfo2)
            }
          }

          else -> throw IllegalStateException()
        }
      }

    val agg1StartBlockNumber = compressionBlobs1.first().blobCounters.startBlockNumber
    val expectedInvalidityProofs = listOf(
      InvalidityProofIndex(
        ftxNumber = 1UL,
        simulatedExecutionBlockNumber = agg1StartBlockNumber - 1UL,
        startBlockTimestamp = Instant.fromEpochSeconds(0),
      ),
    )

    whenever(mockInvalidityProofProvider.getInvalidityProofs(any(), any()))
      .thenReturn(SafeFuture.completedFuture(expectedInvalidityProofs))

    val proofsToAggregate1 =
      ProofsToAggregate(
        compressionProofIndexes =
        compressionBlobs1.map {
          CompressionProofIndex(
            startBlockNumber = it.blobCounters.startBlockNumber,
            endBlockNumber = it.blobCounters.endBlockNumber,
            hash = it.blobCounters.expectedShnarf,
            startBlockTimestamp = it.blobCounters.startBlockTimestamp,
          )
        },
        executionProofs = executionProofs1,
        invalidityProofs = expectedInvalidityProofs,
        parentAggregationLastBlockTimestamp = rollingInfo1.parentAggregationLastBlockTimestamp,
        parentAggregationLastL1RollingHashMessageNumber = rollingInfo1.parentAggregationLastL1RollingHashMessageNumber,
        parentAggregationLastL1RollingHash = rollingInfo1.parentAggregationLastL1RollingHash,
        parentAggregationLastFtxNumber = rollingInfo1.parentAggregationLastFtxNumber,
        parentAggregationLastFtxRollingHash = rollingInfo1.parentAggregationLastFtxRollingHash,
        startBlockTimestamp = compressionBlobs1.first().blobCounters.startBlockTimestamp,
      )

    val proofsToAggregate2 =
      ProofsToAggregate(
        compressionProofIndexes =
        compressionBlobs2.map {
          CompressionProofIndex(
            startBlockNumber = it.blobCounters.startBlockNumber,
            endBlockNumber = it.blobCounters.endBlockNumber,
            hash = it.blobCounters.expectedShnarf,
            startBlockTimestamp = it.blobCounters.startBlockTimestamp,
          )
        },
        executionProofs = executionProofs2,
        invalidityProofs = expectedInvalidityProofs,
        parentAggregationLastBlockTimestamp = rollingInfo2.parentAggregationLastBlockTimestamp,
        parentAggregationLastL1RollingHashMessageNumber = rollingInfo2.parentAggregationLastL1RollingHashMessageNumber,
        parentAggregationLastL1RollingHash = rollingInfo2.parentAggregationLastL1RollingHash,
        parentAggregationLastFtxNumber = rollingInfo2.parentAggregationLastFtxNumber,
        parentAggregationLastFtxRollingHash = rollingInfo2.parentAggregationLastFtxRollingHash,
        startBlockTimestamp = compressionBlobs2.first().blobCounters.startBlockTimestamp,
      )

    val aggregationProof1 = aggregationProofResponse.copy(finalBlockNumber = 23)
    val aggregationProof2 = aggregationProofResponse.copy(finalBlockNumber = 50)

    val aggregation1 =
      Aggregation(
        startBlockNumber = blobsToAggregate1.startBlockNumber,
        endBlockNumber = blobsToAggregate1.endBlockNumber,
        batchCount = compressionBlobs1.sumOf { it.blobCounters.numberOfBatches }.toULong(),
        aggregationProof = aggregationProof1,
      )
    val aggregation1ProofIndex = AggregationProofIndex(
      startBlockNumber = aggregation1.startBlockNumber,
      endBlockNumber = aggregation1.endBlockNumber,
      hash = Random.nextBytes(32),
      startBlockTimestamp = Instant.fromEpochSeconds(0),
    )

    val aggregation2 =
      Aggregation(
        startBlockNumber = blobsToAggregate2.startBlockNumber,
        endBlockNumber = blobsToAggregate2.endBlockNumber,
        batchCount = compressionBlobs2.sumOf { it.blobCounters.numberOfBatches }.toULong(),
        aggregationProof = aggregationProof2,
      )
    val aggregation2ProofIndex = AggregationProofIndex(
      startBlockNumber = aggregation2.startBlockNumber,
      endBlockNumber = aggregation2.endBlockNumber,
      hash = Random.nextBytes(32),
      startBlockTimestamp = Instant.fromEpochSeconds(0),
    )

    whenever(mockProofAggregationClient.createProofRequest(any()))
      .thenAnswer {
        if (it.getArgument<ProofsToAggregate>(0) == proofsToAggregate1) {
          SafeFuture.completedFuture(aggregation1ProofIndex)
        } else if (it.getArgument<ProofsToAggregate>(0) == proofsToAggregate2) {
          SafeFuture.completedFuture(aggregation2ProofIndex)
        } else {
          throw IllegalStateException()
        }
      }

    whenever(mockProofAggregationClient.findProofResponse(any<AggregationProofIndex>()))
      .thenAnswer {
        val proofIndex = it.getArgument<AggregationProofIndex>(0)
        when (proofIndex) {
          aggregation1ProofIndex -> SafeFuture.completedFuture(aggregationProof1)
          aggregation2ProofIndex -> SafeFuture.completedFuture(aggregationProof2)
          else -> SafeFuture.completedFuture(null)
        }
      }

    whenever(
      mockAggregationsRepository.findHighestConsecutiveEndBlockNumber(
        aggregation1.startBlockNumber.toLong(),
      ),
    )
      .thenReturn(
        SafeFuture.completedFuture(aggregation1.endBlockNumber.toLong()),
      )

    whenever(
      mockAggregationsRepository.findHighestConsecutiveEndBlockNumber(
        aggregation2.startBlockNumber.toLong(),
      ),
    )
      .thenReturn(
        SafeFuture.completedFuture(aggregation2.endBlockNumber.toLong()),
      )

    whenever(
      mockAggregationsRepository.saveNewAggregation(
        argThat<Aggregation> {
          this == aggregation1 || this == aggregation2
        },
      ),
    )
      .thenReturn(SafeFuture.completedFuture(Unit))

    // First aggregation should Trigger
    proofAggregationCoordinatorService.action().get()
    await()
      .pollInterval(100.milliseconds.toJavaDuration())
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(meterRegistry.summary("aggregation.blocks.size").count()).isEqualTo(1)
        assertThat(meterRegistry.summary("aggregation.batches.size").count()).isEqualTo(1)
        assertThat(meterRegistry.summary("aggregation.blobs.size").count()).isEqualTo(1)
        assertThat(meterRegistry.summary("aggregation.blocks.size").max()).isEqualTo(23.0)
        assertThat(meterRegistry.summary("aggregation.batches.size").max()).isEqualTo(6.0)
        assertThat(meterRegistry.summary("aggregation.blobs.size").max()).isEqualTo(2.0)
        verify(mockProofAggregationClient).createProofRequest(proofsToAggregate1)
        verify(mockProofAggregationClient).findProofResponse(aggregation1ProofIndex)
        verify(mockAggregationsRepository).saveNewAggregation(aggregation1)
        assertThat(provenAggregation).isEqualTo(aggregation1.endBlockNumber)
        assertThat(aggregationProofRequestCaptures).hasSize(1)
        assertThat(aggregationProofRequestCaptures.single().first).isEqualTo(aggregation1ProofIndex)
        assertThat(aggregationProofRequestCaptures.single().second).isEqualTo(
          Aggregation(
            startBlockNumber = blobsToAggregate1.startBlockNumber,
            endBlockNumber = blobsToAggregate1.endBlockNumber,
            batchCount = compressionBlobs1.sumOf { it.blobCounters.numberOfBatches }.toULong(),
            aggregationProof = null,
          ),
        )
      }

    // Second aggregation should Trigger
    proofAggregationCoordinatorService.action().get()

    await()
      .pollInterval(100.milliseconds.toJavaDuration())
      .atMost(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(meterRegistry.summary("aggregation.blocks.size").count()).isEqualTo(2)
        assertThat(meterRegistry.summary("aggregation.batches.size").count()).isEqualTo(2)
        assertThat(meterRegistry.summary("aggregation.blobs.size").count()).isEqualTo(2)
        assertThat(meterRegistry.summary("aggregation.blocks.size").max()).isEqualTo(27.0)
        assertThat(meterRegistry.summary("aggregation.batches.size").max()).isEqualTo(6.0)
        assertThat(meterRegistry.summary("aggregation.blobs.size").max()).isEqualTo(2.0)
        verify(mockProofAggregationClient).createProofRequest(proofsToAggregate2)
        verify(mockProofAggregationClient).findProofResponse(aggregation2ProofIndex)
        verify(mockAggregationsRepository).saveNewAggregation(aggregation2)
        assertThat(provenAggregation).isEqualTo(aggregation2.endBlockNumber)
        assertThat(aggregationProofRequestCaptures).hasSize(2)
        assertThat(aggregationProofRequestCaptures[1].first).isEqualTo(aggregation2ProofIndex)
        assertThat(aggregationProofRequestCaptures[1].second).isEqualTo(
          Aggregation(
            startBlockNumber = blobsToAggregate2.startBlockNumber,
            endBlockNumber = blobsToAggregate2.endBlockNumber,
            batchCount = compressionBlobs2.sumOf { it.blobCounters.numberOfBatches }.toULong(),
            aggregationProof = null,
          ),
        )
      }
  }
}
