package build.linea.domain

import net.consensys.isSortedBy
import net.consensys.linea.CommonDomainFunctions

/**
 * Represents a block interval, with inclusive start and end block numbers
 * @property startBlockNumber start block number, inclusive
 * @property endBlockNumber end block number, inclusive
 */
interface BlockInterval {
  val startBlockNumber: ULong
  val endBlockNumber: ULong
  val blocksRange: ULongRange
    get() = startBlockNumber..endBlockNumber

  fun intervalString(): String = CommonDomainFunctions.blockIntervalString(startBlockNumber, endBlockNumber)
  fun contains(blockNumber: ULong): Boolean = blockNumber >= startBlockNumber && blockNumber <= endBlockNumber

  companion object {
    operator fun invoke(
      startBlockNumber: ULong,
      endBlockNumber: ULong
    ): BlockInterval {
      return BlockIntervalData(startBlockNumber, endBlockNumber)
    }

    operator fun invoke(
      startBlockNumber: Number,
      endBlockNumber: Number
    ): BlockInterval {
      assert(startBlockNumber.toLong() >= 0 && endBlockNumber.toLong() >= 0) {
        "startBlockNumber=${startBlockNumber.toLong()} and " +
          "endBlockNumber=${endBlockNumber.toLong()} must be non-negative!"
      }
      return BlockIntervalData(startBlockNumber.toLong().toULong(), endBlockNumber.toLong().toULong())
    }

    // Todo: remove later
    /**
     * Please use BlockInterval(startBlockNumber, endBlockNumber) instead
     */
    fun between(
      startBlockNumber: ULong,
      endBlockNumber: ULong
    ): BlockInterval {
      return BlockIntervalData(startBlockNumber, endBlockNumber)
    }
  }
}

/**
 * Represents a block interval
 * @property startBlockNumber starting block number inclusive
 * @property endBlockNumber ending block number inclusive
 */
data class BlockIntervalData(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong
) : BlockInterval {
  init {
    require(startBlockNumber <= endBlockNumber) {
      "startBlockNumber=$startBlockNumber must be less than or equal to endBlockNumber$endBlockNumber"
    }
  }
}

fun List<BlockInterval>.toBlockIntervalsString(): String {
  return this.joinToString(
    separator = ", ",
    prefix = "[",
    postfix = "]$size",
    transform = BlockInterval::intervalString
  )
}

fun <T : BlockInterval> List<T>.filterOutWithEndBlockNumberBefore(
  endBlockNumberInclusive: ULong
): List<T> {
  return this.filter { int -> int.endBlockNumber > endBlockNumberInclusive }
}

fun assertConsecutiveIntervals(intervals: List<BlockInterval>) {
  require(intervals.isSortedBy { it.startBlockNumber }) { "Intervals must be sorted by startBlockNumber" }
  require(intervals.zipWithNext().all { (a, b) -> a.endBlockNumber + 1u == b.startBlockNumber }) {
    "Intervals must be consecutive: intervals=${intervals.toBlockIntervalsString()}"
  }
}

/**
 * Data class that represents sequential blocks intervals for either Conflations, Blobs or Aggregations.
 * Example:
 *  conflations: [100..110], [111..120], [121..130] --> BlockIntervals(100, [110, 120, 130])
 *  Blobs with
 *   Blob1 2 conflations above: [100..110], [111..120]
 *   Blob2 1 conflations:  [121..130]
 *   --> BlockIntervals(100, [120, 130])
 */
data class BlockIntervals(
  val startingBlockNumber: ULong,
  val upperBoundaries: List<ULong>
) {
  // This default constructor is to avoid the parse error when deserializing
  constructor() : this(0UL, listOf())

  fun toIntervalList(): List<BlockInterval> {
    var previousBlockNumber = startingBlockNumber
    val intervals = mutableListOf<BlockInterval>()
    upperBoundaries.forEach {
      intervals.add(BlockIntervalData(previousBlockNumber, it))
      previousBlockNumber = it + 1u
    }
    return intervals
  }

  fun toBlockInterval(): BlockInterval {
    return BlockIntervalData(startingBlockNumber, upperBoundaries.last())
  }
}

fun List<BlockInterval>.toBlockIntervals(): BlockIntervals {
  require(isNotEmpty()) { "BlockIntervals list must not be empty" }
  return BlockIntervals(
    startingBlockNumber = first().startBlockNumber,
    upperBoundaries = map { it.endBlockNumber }
  )
}
