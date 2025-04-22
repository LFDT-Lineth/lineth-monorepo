package linea.ethapi.cursor

import linea.SearchDirection

internal fun rangeChunks(
  start: ULong,
  end: ULong,
  chunkSize: Int
): List<ULongRange> {
  return (start..end step chunkSize.toLong())
    .map { chunkStart ->
      val chunkEnd = (chunkStart + chunkSize.toUInt() - 1u).coerceAtMost(end)
      ULongRange(chunkStart, chunkEnd)
    }
}

/**
 * SearchCursor is a helper class to iterate over a range of ULong values in a binary search manner.
 * When search is not provided returns the next unsearched chunk without moving left or right cursors because
 * no direction is provided, but flags the chunk as searched to not go over again.
 *
 * This caveat is because when searching for EthLogs on L1, a blockInterval (represend by chunk here) may not have logs
 * so the search predicate cannot tell what direction to go next, so we need to flag the chunk as searched and try the
 * next chunk.
 */
internal class BinarySearchCursor(
  val from: ULong,
  val to: ULong,
  val chunkSize: Int
) {
  init {
    require(from <= to) { "invalid range: from=$from must be less or equal to=$to" }
    require(chunkSize > 0) { "chunkSize=$chunkSize must be greater than 0" }
  }
  private data class Chunk(val interval: Pair<ULong, ULong>, var searched: Boolean = false)

  private val searchChunks = rangeChunks(from, to, chunkSize)
    .map { chunkInterval -> Chunk(chunkInterval.first to chunkInterval.endInclusive, searched = false) }
  private var left = 0
  private var right = searchChunks.size - 1
  private var prevCursor: Int? = null

  @Synchronized
  fun next(searchDirection: SearchDirection?): Pair<ULong, ULong>? {
    return if (left > right) {
      null
    } else {
      if (prevCursor == null) {
        // 1st call, lets start in the middle
        val mid = left + (right - left) / 2
        searchChunks[mid] to mid
      } else {
        if (searchDirection == null) {
          findRightNextUnsearchedChunkAndUpdateRightLimit()
        } else {
          if (searchDirection == SearchDirection.FORWARD) {
            left = prevCursor!! + 1
          } else {
            right = prevCursor!! - 1
          }
          if (left > right) {
            null
          } else {
            val mid = left + (right - left) / 2
            val chunk = searchChunks[mid]
            if (chunk.searched) {
              // we have already searched this chunk, lets find next unsearched
              findRightNextUnsearchedChunkAndUpdateRightLimit()
            } else {
              chunk to mid
            }
          }
        }
      }?.let { (chunk, index) ->
        prevCursor = index
        chunk.searched = true
        chunk.interval
      }
    }
  }

  private fun findRightNextUnsearchedChunkAndUpdateRightLimit(): Pair<Chunk, Int>? {
    for (i in right downTo left) {
      if (!searchChunks[i].searched) {
        right = i
        return searchChunks[i] to i
      }
    }
    return null
  }
}
