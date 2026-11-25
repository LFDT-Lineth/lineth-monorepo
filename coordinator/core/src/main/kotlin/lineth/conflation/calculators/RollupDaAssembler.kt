package lineth.conflation.calculators

import linea.domain.BlocksConflation
import linea.kotlin.encodeHex
import lineth.da.DaStreamPosition
import lineth.da.SealedBlob
import lineth.da.SegmentEnd

/**
 * The DA part of a rollup proof request: the blobs the rollup touches, where it starts in the first one, and the end
 * values the rollup guest is expected to output.
 *
 * - [parentSharedBlobCarryOverTailBytes] = 0: the rollup starts on a fresh blob and [parentDataRollingHash] is the
 *   rolling hash before its first blob. [parentBlobDataRollingHash] then equals [parentDataRollingHash]; the guest does
 *   not read it, and the request omits it.
 * - [parentSharedBlobCarryOverTailBytes] > 0: the rollup starts inside its first blob, shared with the previous
 *   rollup. Then [parentDataRollingHash] is the rolling hash after that blob, and [parentBlobDataRollingHash] the one
 *   before it, which opens the preimage of [parentDataRollingHash].
 *
 * Blobs are shared by reference: a shared blob is the same [SealedBlob] object in both witnesses.
 */
class RollupDaWitness(
  val blobs: List<SealedBlob>,
  val parentDataRollingHash: ByteArray,
  val parentSharedBlobCarryOverTailBytes: UInt,
  val parentBlobDataRollingHash: ByteArray,
  val expectedEndDataRollingHash: ByteArray,
  val expectedFinalSharedBlobCarryOverTailBytes: UInt,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupDaWitness

    if (blobs != other.blobs) return false
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (parentSharedBlobCarryOverTailBytes != other.parentSharedBlobCarryOverTailBytes) return false
    if (!parentBlobDataRollingHash.contentEquals(other.parentBlobDataRollingHash)) return false
    if (!expectedEndDataRollingHash.contentEquals(other.expectedEndDataRollingHash)) return false
    if (expectedFinalSharedBlobCarryOverTailBytes != other.expectedFinalSharedBlobCarryOverTailBytes) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blobs.hashCode()
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + parentSharedBlobCarryOverTailBytes.hashCode()
    result = 31 * result + parentBlobDataRollingHash.contentHashCode()
    result = 31 * result + expectedEndDataRollingHash.contentHashCode()
    result = 31 * result + expectedFinalSharedBlobCarryOverTailBytes.hashCode()
    return result
  }

  override fun toString(): String {
    return "RollupDaWitness(blobs=${blobs.size}, parentDataRollingHash=${parentDataRollingHash.encodeHex()}, " +
      "parentSharedBlobCarryOverTailBytes=$parentSharedBlobCarryOverTailBytes, " +
      "parentBlobDataRollingHash=${parentBlobDataRollingHash.encodeHex()}, " +
      "expectedEndDataRollingHash=${expectedEndDataRollingHash.encodeHex()}, " +
      "expectedFinalSharedBlobCarryOverTailBytes=$expectedFinalSharedBlobCarryOverTailBytes)"
  }
}

/** A rollup cut by a [RollupCalculatorV2]: its conflations, in order, and its DA witness. */
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
  private var parentCarryOver = start.sharedBlobCarryOverTailBytes
  private var parent = start.dataRollingHash

  // Rolling hash before the open rollup's first blob: opens the preimage of `parent` when parentCarryOver > 0, equals
  // it otherwise.
  private var parentBlobDataRollingHash: ByteArray = start.parentBlobDataRollingHash

  /** The open rollup's conflations, oldest first. */
  val openConflations: List<BlocksConflation> get() = conflations

  fun addBlob(blob: SealedBlob) {
    val previous = blobs.lastOrNull()
    if (previous != null) {
      check(blob.parentDataRollingHash.contentEquals(previous.dataRollingHash)) {
        "blob does not continue the open rollup: parentDataRollingHash=${blob.parentDataRollingHash.encodeHex()} " +
          "but the previous blob ends at ${previous.dataRollingHash.encodeHex()}"
      }
    } else if (parentCarryOver > 0u) {
      // A rollup starting inside a shared blob: its first blob must be that shared blob.
      check(
        blob.dataRollingHash.contentEquals(parent) &&
          blob.parentDataRollingHash.contentEquals(parentBlobDataRollingHash),
      ) {
        "first blob of a rollup starting inside a shared blob must be that blob: " +
          "expected dataRollingHash=${parent.encodeHex()}, got ${blob.dataRollingHash.encodeHex()}"
      }
    } else {
      check(blob.parentDataRollingHash.contentEquals(parent)) {
        "first blob of the rollup must follow parentDataRollingHash=${parent.encodeHex()}, " +
          "got parentDataRollingHash=${blob.parentDataRollingHash.encodeHex()}"
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
    val tail = at.unusedTailBytes
    val witness = RollupDaWitness(
      blobs = blobs.toList(),
      parentDataRollingHash = parent,
      parentSharedBlobCarryOverTailBytes = parentCarryOver,
      parentBlobDataRollingHash = parentBlobDataRollingHash,
      expectedEndDataRollingHash = last.dataRollingHash,
      expectedFinalSharedBlobCarryOverTailBytes = tail,
    )
    blobs.clear()
    // The next rollup starts where this one ends: inside `last` if bytes remain (the same object then sits in both
    // witnesses), else on a fresh blob.
    if (tail > 0u) blobs += last
    parentCarryOver = tail
    parent = last.dataRollingHash
    parentBlobDataRollingHash = if (tail > 0u) last.parentDataRollingHash else last.dataRollingHash
    val rollup = Rollup(conflations.toList(), witness)
    conflations.clear()
    return rollup
  }
}
