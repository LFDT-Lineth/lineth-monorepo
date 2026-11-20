package lineth.conflation.calculators

import linea.da.DaCompressor
import linea.da.DaConstants.BLOB_PAYLOAD_CAPACITY
import linea.da.DaConstants.SEGMENT_LENGTH_PREFIX_BYTES
import linea.da.DaStream
import linea.da.SealedBlob
import linea.domain.BlocksConflation
import linea.kotlin.toInstantFromEpochSeconds
import lineth.encoding.BlockEncoder
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import kotlin.time.Duration
import kotlin.time.Instant

/**
 * Decides RISC-V blob cut-offs: compresses conflations with the [DaCompressor], appends them to the [DaStream] in
 * order, flushes the open blob on blob triggers, and emits one [BlobEvent] per sealed blob, carrying every conflation
 * whose segment ends in it.
 *
 * Segments end in the order conflations were appended, so a sealed blob's cut points belong to the oldest queued
 * conflations, one to one.
 *
 * Resuming inside a shared blob: the last proven rollup ended inside a blob that is already sealed, stored and folded
 * into that rollup's `endDataRollingHash`. [daStream] reopens it with the bytes before the rollup's cut in place, and the
 * next appended segments must fill its last `sharedBlobCarryOverTailBytes` exactly as before, or the stream rejects them
 * with `SEEDED_BLOB_MISMATCH`. Until they are filled, the blob cannot be sealed early: [tryFlush] declines.
 *
 * Not thread-safe: all calls must be confined to the thread that owns [daStream].
 *
 * @param blobDeadline flushes the open blob once its oldest conflation's first block is this old; see [checkDeadline].
 *   It bounds the data's age from the block timestamp, and conflations arrive only once their execution proof is done,
 *   so it must be well above the execution proving time: otherwise every blob is flushed with a single conflation.
 * @param sharedBlobCarryOverTailBytes the `sharedBlobCarryOverTailBytes` [daStream] was opened with: > 0 when it
 *   resumes inside a shared blob, see above.
 */
class GlobalBlobCalculator(
  private val compressor: DaCompressor,
  private val daStream: DaStream,
  private val triggers: List<BlobTriggerCalculator>,
  private val blockEncoder: BlockEncoder,
  private val handler: BlobHandler,
  private val blobDeadline: Duration? = null,
  sharedBlobCarryOverTailBytes: Int = 0,
  private val log: Logger = LogManager.getLogger(GlobalBlobCalculator::class.java),
) {
  /** Appended to the stream, but the blob holding the end of their segment is not sealed yet. */
  private val appended = ArrayDeque<CompressedBlocksConflation>()

  /** Carry-over tail bytes of the resumed shared blob that appended segments have not filled yet; > 0 blocks flushing. */
  private var remainingSharedBlobCarryOverTailBytes = sharedBlobCarryOverTailBytes

  init {
    require(sharedBlobCarryOverTailBytes >= 0) {
      "sharedBlobCarryOverTailBytes=$sharedBlobCarryOverTailBytes must not be negative"
    }
  }

  fun newConflation(conflation: BlocksConflation) {
    triggers.firstNotNullOfOrNull { it.checkOverflow(conflation) }?.let { tryFlush(it) }

    val compressed = compressor.compress(conflation.blocks.map(blockEncoder::encode))
    check(compressed.blockInterval.isSameInterval(conflation)) {
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
    remainingSharedBlobCarryOverTailBytes = maxOf(
      0,
      remainingSharedBlobCarryOverTailBytes - (SEGMENT_LENGTH_PREFIX_BYTES + compressed.compressedData.size),
    )
    // append seals only full blobs; a short one can only be a resumed shared blob that was flushed originally.
    sealed.forEach { emit(it, flushed = it.payloadBytes < BLOB_PAYLOAD_CAPACITY) }

    triggers.forEach { it.appendConflation(conflation) }
    triggers.firstNotNullOfOrNull { it.checkTrigger(conflation) }?.let { tryFlush(it) }
  }

  /**
   * Ends the open blob early: on blob triggers, the blob deadline, and when the rollup calculator needs a rollup cut.
   *
   * @return true if a blob was flushed. False, flushing nothing, when no blob is open or while the carry-over tail of a
   *   resumed shared blob is not filled yet. Once it is filled, flushing re-seals the shared blob unchanged.
   */
  fun tryFlush(trigger: BlobTrigger? = null): Boolean {
    if (remainingSharedBlobCarryOverTailBytes > 0) {
      log.debug(
        "skipping blob flush trigger={}: {} carry-over tail bytes of the resumed shared blob are not filled yet",
        trigger,
        remainingSharedBlobCarryOverTailBytes,
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
    val oldestBlockTimestamp = oldest.conflation.blocks.first().timestamp.toInstantFromEpochSeconds()
    return if (now - oldestBlockTimestamp >= deadline) tryFlush(BlobTrigger.TIME_LIMIT) else false
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
    remainingSharedBlobCarryOverTailBytes = 0
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
