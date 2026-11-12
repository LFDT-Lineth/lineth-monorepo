package lineth.conflation.calculators

import linea.da.DaStreamPosition
import linea.da.SealedBlob
import linea.da.SegmentEnd
import linea.domain.BlocksConflation
import linea.kotlin.encodeHex

/**
 * The DA part of a rollup proof request: the blobs the rollup touches, where it starts in the first one, and the end
 * values the rollup guest is expected to output.
 *
 * - [parentDataTailTakeBytes] = 0: the rollup starts on a fresh blob and [parentDataRollingHash] is the rolling hash
 *   before its first blob.
 * - [parentDataTailTakeBytes] > 0: the rollup starts inside its first blob, shared with the previous rollup. Then
 *   [parentDataRollingHash] is the rolling hash after that blob, and [boundaryPrevDataRollingHash] the one before it.
 *
 * Blobs are shared by reference: a shared blob is the same [SealedBlob] object in both witnesses.
 */
class RollupDaWitness(
  val blobs: List<SealedBlob>,
  val parentDataRollingHash: ByteArray,
  val parentDataTailTakeBytes: Int,
  val boundaryPrevDataRollingHash: ByteArray?,
  val expectedEndDataRollingHash: ByteArray,
  val expectedFinalDataTailDiscardBytes: Int,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupDaWitness

    if (blobs != other.blobs) return false
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (parentDataTailTakeBytes != other.parentDataTailTakeBytes) return false
    if (!boundaryPrevDataRollingHash.contentEquals(other.boundaryPrevDataRollingHash)) return false
    if (!expectedEndDataRollingHash.contentEquals(other.expectedEndDataRollingHash)) return false
    if (expectedFinalDataTailDiscardBytes != other.expectedFinalDataTailDiscardBytes) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blobs.hashCode()
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + parentDataTailTakeBytes
    result = 31 * result + (boundaryPrevDataRollingHash?.contentHashCode() ?: 0)
    result = 31 * result + expectedEndDataRollingHash.contentHashCode()
    result = 31 * result + expectedFinalDataTailDiscardBytes
    return result
  }

  override fun toString(): String {
    return "RollupDaWitness(blobs=${blobs.size}, parentDataRollingHash=${parentDataRollingHash.encodeHex()}, " +
      "parentDataTailTakeBytes=$parentDataTailTakeBytes, " +
      "boundaryPrevDataRollingHash=${boundaryPrevDataRollingHash?.encodeHex()}, " +
      "expectedEndDataRollingHash=${expectedEndDataRollingHash.encodeHex()}, " +
      "expectedFinalDataTailDiscardBytes=$expectedFinalDataTailDiscardBytes)"
  }
}

/** A rollup cut by a [RollupCalculator]: its conflations, in order, and its DA witness. */
data class Rollup(
  val conflations: List<BlocksConflation>,
  val da: RollupDaWitness,
) {
  init {
    require(conflations.isNotEmpty()) { "a rollup must have at least one conflation" }
  }

  val startBlockNumber: ULong get() = conflations.first().startBlockNumber
  val endBlockNumber: ULong get() = conflations.last().endBlockNumber
}

/**
 * The open rollup: its conflations, its blobs and its start values. A cut in the middle of a blob loses nothing: the
 * conflations after the cut go into the next rollup, which starts inside that blob, so the same [SealedBlob] then sits
 * in both rollups.
 *
 * Not thread-safe: confined to the thread that drives [GlobalBlobCalculator].
 */
class RollupDaAssembler(start: DaStreamPosition) {
  private val conflations = mutableListOf<BlocksConflation>()
  private val blobs = mutableListOf<SealedBlob>()

  // Start values of the open rollup.
  private var take = start.tailBytes
  private var parent = start.dataRollingHash

  // Set only when the open rollup starts inside a shared blob (take > 0); the spec requires
  // boundaryPrevDataRollingHash exactly then, to open the preimage of `parent`.
  private var boundaryPrev: ByteArray? = start.prevDataRollingHash.takeIf { start.tailBytes > 0 }

  /** The open rollup's conflations, oldest first. */
  val openConflations: List<BlocksConflation> get() = conflations

  fun addBlob(blob: SealedBlob) {
    val previous = blobs.lastOrNull()
    if (previous != null) {
      check(blob.prevDataRollingHash.contentEquals(previous.dataRollingHash)) {
        "blob does not continue the open rollup: prevDataRollingHash=${blob.prevDataRollingHash.encodeHex()} " +
          "but the previous blob ends at ${previous.dataRollingHash.encodeHex()}"
      }
    } else if (take > 0) {
      // A rollup starting inside a shared blob: its first blob must be that shared blob.
      check(blob.dataRollingHash.contentEquals(parent) && blob.prevDataRollingHash.contentEquals(boundaryPrev)) {
        "first blob of a rollup starting inside a shared blob must be that blob: " +
          "expected dataRollingHash=${parent.encodeHex()}, got ${blob.dataRollingHash.encodeHex()}"
      }
    } else {
      check(blob.prevDataRollingHash.contentEquals(parent)) {
        "first blob of the rollup must follow parentDataRollingHash=${parent.encodeHex()}, " +
          "got prevDataRollingHash=${blob.prevDataRollingHash.encodeHex()}"
      }
    }
    blobs += blob
  }

  fun addConflation(conflation: BlocksConflation) {
    conflations += conflation
  }

  /** Ends the open rollup at [at], a cut in the newest blob, and opens the next rollup where this one ends. */
  fun endRollup(at: SegmentEnd): Rollup {
    val last = blobs.lastOrNull() ?: error("cannot end a rollup without blobs")
    check(at in last.segmentEnds) { "cut=$at is not in the newest blob of the open rollup" }
    check(conflations.lastOrNull()?.endBlockNumber == at.conflationEndBlock) {
      "cut=$at does not end the open rollup's last conflation=${conflations.lastOrNull()?.intervalString()}"
    }
    val tail = at.tailBytes
    val witness = RollupDaWitness(
      blobs = blobs.toList(),
      parentDataRollingHash = parent,
      parentDataTailTakeBytes = take,
      boundaryPrevDataRollingHash = boundaryPrev,
      expectedEndDataRollingHash = last.dataRollingHash,
      expectedFinalDataTailDiscardBytes = tail,
    )
    blobs.clear()
    // The next rollup starts where this one ends: inside `last` if bytes remain (the same object then sits in both
    // witnesses), else on a fresh blob.
    if (tail > 0) blobs += last
    take = tail
    parent = last.dataRollingHash
    boundaryPrev = if (tail > 0) last.prevDataRollingHash else null
    val rollup = Rollup(conflations.toList(), witness)
    conflations.clear()
    return rollup
  }
}
