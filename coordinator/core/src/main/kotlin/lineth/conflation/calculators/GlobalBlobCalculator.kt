package lineth.conflation.calculators

import linea.da.DaConstants.BLOB_PAYLOAD_CAPACITY
import linea.da.DaStream
import linea.da.SealedBlob
import linea.domain.BlocksConflation
import lineth.encoding.BlockEncoder
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.time.Duration
import kotlin.time.Instant

/**
 * Decides RISC-V blob cut-offs: feeds conflations to the [DaStream] in order, flushes the open blob on blob triggers,
 * and emits one [BlobEvent] per sealed blob, carrying every conflation whose frame ends in it.
 *
 * Frames end in the order conflations were appended, so a sealed blob's cut points belong to the oldest queued
 * conflations, one to one.
 *
 * Not thread-safe: all calls must be confined to the thread that owns [daStream].
 *
 * @param blobDeadline flushes the open blob once its oldest conflation's first block is this old; see [checkDeadline].
 *   It bounds the data's age from the block timestamp, and conflations arrive only once their execution proof is done,
 *   so it must be well above the execution proving time: otherwise every blob is flushed with a single conflation.
 * @param sharedBlobTailBytes the `tailBytes` [daStream] was opened with: > 0 when it resumes inside a shared blob. Until
 *   appended frames have reproduced that many bytes, flushing is skipped: a shorter seal would differ from the already
 *   stored and submitted blob.
 */
class GlobalBlobCalculator(
  private val daStream: DaStream,
  private val triggers: List<BlobTriggerCalculator>,
  private val blockEncoder: BlockEncoder,
  private val handler: BlobHandler,
  private val blobDeadline: Duration? = null,
  sharedBlobTailBytes: Int = 0,
  private val log: Logger = LogManager.getLogger(GlobalBlobCalculator::class.java),
) {
  /** Appended to the stream, but the blob holding the end of their frame is not sealed yet. */
  private val appended = ArrayDeque<CompressedBlocksConflation>()

  /** Shared blob bytes still to reproduce; flushing is skipped while > 0. */
  private var bytesToReproduce = sharedBlobTailBytes

  init {
    require(sharedBlobTailBytes >= 0) { "sharedBlobTailBytes=$sharedBlobTailBytes must not be negative" }
  }

  fun newConflation(conflation: BlocksConflation) {
    triggers.firstNotNullOfOrNull { it.checkOverflow(conflation) }?.let { flush(it) }

    val compressed = daStream.compress(conflation.blocks.map(blockEncoder::encode))
    check(
      compressed.blockInterval.startBlockNumber == conflation.startBlockNumber &&
        compressed.blockInterval.endBlockNumber == conflation.endBlockNumber,
    ) {
      "compressed interval=${compressed.blockInterval.intervalString()} differs from " +
        "conflation=${conflation.intervalString()}"
    }
    appended += CompressedBlocksConflation(conflation, compressed)
    val sealed = try {
      daStream.append(compressed)
    } catch (e: Exception) {
      // The stream is unchanged on error, so the conflation is not appended either.
      appended.removeLast()
      throw e
    }
    bytesToReproduce = maxOf(0, bytesToReproduce - compressed.compressedBytes)
    // append seals only full blobs; a short one can only be a resumed shared blob that was flushed originally.
    sealed.forEach { emit(it, flushed = it.payloadBytes < BLOB_PAYLOAD_CAPACITY) }

    triggers.forEach { it.appendConflation(conflation) }
    triggers.firstNotNullOfOrNull { it.checkTrigger(conflation) }?.let { flush(it) }
  }

  /**
   * Ends the open blob early: on blob triggers, the blob deadline, and when the rollup calculator needs a rollup cut.
   *
   * @return true if a blob was flushed. False when no blob is open, or while a resumed shared blob is still being
   *   reproduced. Once it has been reproduced in full, flushing re-seals it unchanged.
   */
  fun flush(trigger: BlobTrigger? = null): Boolean {
    if (bytesToReproduce > 0) {
      log.debug(
        "skipping blob flush trigger={}: {} bytes of the resumed shared blob are still to reproduce",
        trigger,
        bytesToReproduce,
      )
      return false
    }
    val blob = daStream.flush() ?: return false
    log.debug("flushing blob trigger={} payloadBytes={}", trigger, blob.payloadBytes)
    emit(blob, flushed = true)
    return true
  }

  /** Flushes the open blob if its oldest conflation's first block is at least [blobDeadline] old at [now]. */
  fun checkDeadline(now: Instant): Boolean {
    val deadline = blobDeadline ?: return false
    val oldest = appended.firstOrNull() ?: return false
    val oldestBlockTimestamp = Instant.fromEpochSeconds(oldest.conflation.blocks.first().timestamp.toLong())
    return if (now - oldestBlockTimestamp >= deadline) flush(BlobTrigger.TIME_LIMIT) else false
  }

  private fun emit(blob: SealedBlob, flushed: Boolean) {
    val sealedConflations = blob.segmentEnds.map { end ->
      val oldest = appended.removeFirstOrNull()
        ?: error("blob has a cut at block ${end.conflationEndBlock} but no conflation is queued")
      check(oldest.conflation.endBlockNumber == end.conflationEndBlock) {
        "blob cut at block ${end.conflationEndBlock} does not match the oldest queued " +
          "conflation=${oldest.conflation.intervalString()}"
      }
      SealedConflation(oldest, end)
    }
    if (flushed && sealedConflations.isEmpty()) error("a flushed blob must end at least one conflation")
    bytesToReproduce = 0
    triggers.forEach { it.reset() } // a new blob opens
    log.debug(
      "blob sealed: payloadBytes={} flushed={} conflations={}",
      blob.payloadBytes,
      flushed,
      sealedConflations.map { it.conflation.conflation.intervalString() },
    )
    handler.onBlob(BlobEvent(blob, sealedConflations, appended.firstOrNull(), flushed))
  }
}
