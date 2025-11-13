package linea.staterecovery.datafetching

import io.vertx.core.Vertx
import linea.staterecovery.BlobDecompressorAndDeserializer
import linea.staterecovery.BlockFromL1RecoveredData
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.function.Supplier
import kotlin.time.Duration

internal class BlobDecompressionTask(
  private val vertx: Vertx,
  private val pollingInterval: Duration,
  private val blobDecompressor: BlobDecompressorAndDeserializer,
  private val rawBlobsQueue: ConcurrentLinkedQueue<SubmissionEventsAndData<ByteArray>>,
  private val decompressedBlocksQueue: ConcurrentLinkedQueue<SubmissionEventsAndData<BlockFromL1RecoveredData>>,
  private val decompressedFinalizationQueueLimit: Supplier<Int>,
  private val log: Logger = LogManager.getLogger(SubmissionsFetchingTask::class.java),
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds,
  log = log,
  name = "BlobDecompressionTask",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  override fun action(): SafeFuture<*> {
    return decompressAndDeserializeBlobs()
  }

  private fun decompressAndDeserializeBlobs(): SafeFuture<Unit> {
    if (decompressedBlocksQueue.size >= decompressedFinalizationQueueLimit.get()) {
      return SafeFuture.completedFuture(Unit)
    }
    val submissionEventsAndData = rawBlobsQueue.poll()
      ?: return SafeFuture.completedFuture(Unit)

    return blobDecompressor
      .decompress(
        startBlockNumber = submissionEventsAndData.submissionEvents.dataFinalizedEvent.event.startBlockNumber,
        blobs = submissionEventsAndData.data,
      ).thenCompose { decompressedBlocks ->
        decompressedBlocksQueue.add(
          SubmissionEventsAndData(submissionEventsAndData.submissionEvents, decompressedBlocks),
        )
        decompressAndDeserializeBlobs()
      }
  }
}
