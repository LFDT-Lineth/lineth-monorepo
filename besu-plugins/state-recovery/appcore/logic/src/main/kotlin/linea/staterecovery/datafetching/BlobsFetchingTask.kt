package linea.staterecovery.datafetching

import io.vertx.core.Vertx
import linea.staterecovery.BlobFetcher
import linea.staterecovery.FinalizationAndDataEventsV3
import linea.staterecovery.TransactionDetailsClient
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedQueue
import kotlin.time.Duration

internal class BlobsFetchingTask(
  val vertx: Vertx,
  val pollingInterval: Duration,
  private val blobsFetcher: BlobFetcher,
  private val transactionDetailsClient: TransactionDetailsClient,
  private val submissionEventsQueue: ConcurrentLinkedQueue<FinalizationAndDataEventsV3>,
  private val compressedBlobsQueue: ConcurrentLinkedQueue<SubmissionEventsAndData<ByteArray>>,
  private val compressedBlobsQueueLimit: Int,
  private val log: Logger = LogManager.getLogger(BlobsFetchingTask::class.java),
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds,
  log = log,
  name = "BlobsFetchingTask",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {

  override fun action(): SafeFuture<*> {
    return fetchBlobs()
  }

  private fun fetchBlobs(): SafeFuture<*> {
    if (compressedBlobsQueue.size >= compressedBlobsQueueLimit) {
      // Queue is full, no need to fetch more
      return SafeFuture.completedFuture(Unit)
    }
    val nextSubmission = submissionEventsQueue.peek()
    if (nextSubmission == null) {
      // No more submissions to fetch
      return SafeFuture.completedFuture(Unit)
    }

    return fetchBlobsOfSubmissionEvents(nextSubmission)
      .thenCompose { blobs ->
        compressedBlobsQueue.add(SubmissionEventsAndData(nextSubmission, blobs))
        submissionEventsQueue.poll()
        fetchBlobs()
      }
  }

  private fun fetchBlobsOfSubmissionEvents(
    submissionEvents: FinalizationAndDataEventsV3,
  ): SafeFuture<List<ByteArray>> {
    return SafeFuture.collectAll(
      submissionEvents.dataSubmittedEvents
        .map {
          transactionDetailsClient.getBlobVersionedHashesByTransactionHash(it.log.transactionHash)
        }.stream(),
    )
      .thenCompose { blobsVersionedHashesByTransaction ->
        blobsFetcher.fetchBlobsByHash(blobsVersionedHashesByTransaction.flatten())
      }
  }
}
