package linea.staterecovery

import build.linea.domain.BlockInterval
import io.vertx.core.Vertx
import linea.staterecovery.datafetching.SubmissionEventsAndData
import linea.staterecovery.datafetching.SubmissionsFetchingTask
import net.consensys.linea.BlockParameter.Companion.toBlockParameter
import net.consensys.linea.async.AsyncRetryer
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.seconds

class LookBackBlockHashesFetcher(
  private val vertx: Vertx,
  private val elClient: ExecutionLayerClient,
  private val submissionsFetcher: SubmissionsFetchingTask
) {
  fun getLookBackHashes(
    status: StateRecoveryStatus
  ): SafeFuture<Map<ULong, ByteArray>> {
    val intervals = lookbackFetchingIntervals(
      headBlockNumber = status.headBlockNumber,
      recoveryStartBlockNumber = status.stateRecoverStartBlockNumber,
      lookbackWindow = 256UL
    )

    return SafeFuture.collectAll(
      listOf(
        intervals.elInterval?.let(::getLookBackHashesFromLocalEl) ?: SafeFuture.completedFuture(emptyMap()),
        intervals.l1Interval?.let(::getLookBackHashesFromL1) ?: SafeFuture.completedFuture(emptyMap())
      ).stream()
    )
      .thenApply { (blockHashesFromEl, blockHashesFromL1) -> blockHashesFromEl + blockHashesFromL1 }
  }

  fun getLookBackHashesFromLocalEl(
    blockInterval: BlockInterval
  ): SafeFuture<Map<ULong, ByteArray>> {
    return SafeFuture
      .collectAll(blockInterval.blocksRange.map { elClient.getBlockNumberAndHash(it.toBlockParameter()) }.stream())
      .thenApply { blockNumbersAndHashes ->
        blockNumbersAndHashes.associate { (blockNumber, blockHash) -> blockNumber to blockHash }
      }
  }

  fun getLookBackHashesFromL1(
    blockInterval: BlockInterval
  ): SafeFuture<Map<ULong, ByteArray>> {
    return AsyncRetryer.retry(
      vertx,
      backoffDelay = 1.seconds,
      stopRetriesPredicate = { submissions ->
        submissions.isNotEmpty() &&
          submissions.last().submissionEvents.dataFinalizedEvent.event.endBlockNumber >= blockInterval.endBlockNumber
      }
    ) {
      // get the data without removing it from the queue
      // it must still be in the queue until is imported to the EL
      val availableSubmissions = submissionsFetcher.decompressedBlocksQueue.toList()
      if (shallIncreaseQueueLimit(availableSubmissions, blockInterval)) {
        // Queue limit is bit enough to fetch all the blocks in the interval
        // this can happen if queueLimit is small and aggrgations are very small as well
        submissionsFetcher.incrementDecompressedBlobsQueueLimit(incrementFactor = 2)
      }
      SafeFuture.completedFuture(availableSubmissions)
    }
      .thenApply { submissions ->
        // Reset the queue limit to the original size
        submissionsFetcher.resetDecompressedBlobsQueueLimitToOriginalSize()
        submissions
          .flatMap { submission -> submission.data }
          .associate { block -> block.header.blockNumber to block.header.blockHash }
          .filter { (blockNumber, _) -> blockNumber in blockInterval.blocksRange }
      }
  }

  fun shallIncreaseQueueLimit(
    availableSubmissions: List<SubmissionEventsAndData<BlockFromL1RecoveredData>>,
    blockInterval: BlockInterval
  ): Boolean {
    if (availableSubmissions.isEmpty()) {
      return false
    }
    if (
      availableSubmissions.size >= submissionsFetcher.decompressedBlobsQueueLimit &&
      availableSubmissions.last()
        .submissionEvents.dataFinalizedEvent.event.endBlockNumber < blockInterval.endBlockNumber
    ) {
      return true
    }
    return false
  }
}
