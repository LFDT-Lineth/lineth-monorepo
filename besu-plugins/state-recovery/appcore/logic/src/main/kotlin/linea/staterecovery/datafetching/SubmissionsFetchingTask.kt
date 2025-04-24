package linea.staterecovery.datafetching

import io.vertx.core.Vertx
import linea.domain.BlockParameter
import linea.staterecovery.BlobDecompressorAndDeserializer
import linea.staterecovery.BlobFetcher
import linea.staterecovery.BlockFromL1RecoveredData
import linea.staterecovery.FinalizationAndDataEventsV3
import linea.staterecovery.LineaRollupSubmissionEventsClient
import linea.staterecovery.TransactionDetailsClient
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

/**
 * This class is responsible for fetching blobs from the L1 and decompressing them.
 * In an async and decoupled way.
 *
 * It relies on 2 background loops:
 *  1. Fetch blobs from L1 and store them in a queue
 *  2. Decompress and deserialize fetched blobs and store them in a queue
 *
 */
data class SubmissionEventsAndData<T>(
  val submissionEvents: FinalizationAndDataEventsV3,
  val data: List<T>
)

class SubmissionsFetchingTask(
  private val vertx: Vertx,
  private val l1EarliestBlockWithFinalizationThatSupportRecovery: BlockParameter,
  private val l1PollingInterval: Duration,
  private val l2StartBlockNumberToFetchInclusive: ULong,
  private val submissionEventsClient: LineaRollupSubmissionEventsClient,
  private val blobsFetcher: BlobFetcher,
  private val transactionDetailsClient: TransactionDetailsClient,
  private val blobDecompressor: BlobDecompressorAndDeserializer,
  private val submissionEventsQueueLimit: Int,
  private val compressedBlobsQueueLimit: Int,
  private val targetDecompressedBlobsQueueLimit: Int,
  private val debugForceSyncStopBlockNumber: ULong?,
  private val log: Logger = LogManager.getLogger(SubmissionsFetchingTask::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = l1PollingInterval.inWholeMilliseconds,
  log = log
) {
  init {
    require(submissionEventsQueueLimit >= 1) {
      "submissionEventsQueueLimit=$submissionEventsQueueLimit must be greater than zero"
    }
    require(compressedBlobsQueueLimit >= 1) {
      "compressedBlobsQueueLimit=$compressedBlobsQueueLimit must be greater than zero"
    }
    require(targetDecompressedBlobsQueueLimit >= 1) {
      "targetDecompressedBlobsQueueLimit=$targetDecompressedBlobsQueueLimit must be greater than zero"
    }
  }
  private val dynamicDecompressedBlobsQueueLimit = AtomicInteger(targetDecompressedBlobsQueueLimit)
  val decompressedBlobsQueueLimit: Int
    get() = dynamicDecompressedBlobsQueueLimit.get()
  fun incrementDecompressedBlobsQueueLimit(incrementFactor: Int) {
    dynamicDecompressedBlobsQueueLimit.accumulateAndGet(incrementFactor) { prev, _ -> prev + incrementFactor }
  }
  fun resetDecompressedBlobsQueueLimitToOriginalSize() {
    dynamicDecompressedBlobsQueueLimit.set(targetDecompressedBlobsQueueLimit)
  }

  private val submissionEventsQueue = ConcurrentLinkedQueue<FinalizationAndDataEventsV3>()
  private val compressedBlobsQueue = ConcurrentLinkedQueue<SubmissionEventsAndData<ByteArray>>()
  val decompressedBlocksQueue: ConcurrentLinkedQueue<SubmissionEventsAndData<BlockFromL1RecoveredData>> =
    ConcurrentLinkedQueue<SubmissionEventsAndData<BlockFromL1RecoveredData>>()

  private val submissionEventsFetchingTask = SubmissionEventsFetchingTask(
    vertx = vertx,
    l1EarliestBlockWithFinalizationThatSupportRecovery = l1EarliestBlockWithFinalizationThatSupportRecovery,
    l1PollingInterval = l1PollingInterval,
    submissionEventsClient = submissionEventsClient,
    l2StartBlockNumber = l2StartBlockNumberToFetchInclusive,
    submissionEventsQueue = submissionEventsQueue,
    queueLimit = submissionEventsQueueLimit,
    debugForceSyncStopBlockNumber = debugForceSyncStopBlockNumber
  )
  private val blobFetchingTask = BlobsFetchingTask(
    vertx = vertx,
    pollingInterval = 1.seconds,
    submissionEventsQueue = submissionEventsQueue,
    blobsFetcher = blobsFetcher,
    transactionDetailsClient = transactionDetailsClient,
    compressedBlobsQueue = compressedBlobsQueue,
    compressedBlobsQueueLimit = compressedBlobsQueueLimit
  )
  private val blobDecompressionTask = BlobDecompressionTask(
    vertx = vertx,
    pollingInterval = 1.seconds,
    blobDecompressor = blobDecompressor,
    rawBlobsQueue = compressedBlobsQueue,
    decompressedBlocksQueue = decompressedBlocksQueue,
    decompressedFinalizationQueueLimit = dynamicDecompressedBlobsQueueLimit::get
  )

  @Synchronized
  override fun start(): SafeFuture<Unit> {
    return SafeFuture.allOf(
      submissionEventsFetchingTask.start(),
      blobFetchingTask.start(),
      blobDecompressionTask.start()
    ).thenCompose { super.start() }
  }

  @Synchronized
  override fun stop(): SafeFuture<Unit> {
    return SafeFuture.allOf(
      submissionEventsFetchingTask.stop(),
      blobFetchingTask.stop(),
      blobDecompressionTask.stop()
    ).thenCompose { super.stop() }
  }

  override fun action(): SafeFuture<*> {
    return SafeFuture.completedFuture(Unit)
  }

  @Synchronized
  fun peekNextFinalizationReadyToImport(): SubmissionEventsAndData<BlockFromL1RecoveredData>? {
    return decompressedBlocksQueue.peek()
  }

  @Synchronized
  fun finalizationsReadyToImport(): Int = decompressedBlocksQueue.size

  @Synchronized
  fun pruneQueueForElementsUpToInclusive(
    elHeadBlockNumber: ULong
  ) {
    decompressedBlocksQueue.removeIf {
      it.submissionEvents.dataFinalizedEvent.event.endBlockNumber <= elHeadBlockNumber
    }
  }
}
