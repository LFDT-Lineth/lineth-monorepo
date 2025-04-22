package linea.ethapi.cursor

import linea.SearchDirection

internal class ConsecutiveSearchCursor(
  val from: ULong,
  val to: ULong,
  val chunkSize: Int,
  val direction: SearchDirection
) : Iterator<ULongRange> {

  private val chunks: List<ULongRange> = run {
    val list = rangeChunks(from, to, chunkSize)
    if (direction == SearchDirection.FORWARD) list else list.reversed()
  }

  private var currentIndex = 0

  @Synchronized
  override fun hasNext(): Boolean = currentIndex < chunks.size

  @Synchronized
  override fun next(): ULongRange {
    if (!hasNext()) throw NoSuchElementException("No more chunks available.")
    return chunks[currentIndex++]
  }
}
