package linea.ethapi.cursor

import linea.SearchDirection
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class ConsecutiveSearchCursorTest {

  @Test
  fun `should iterate over chunks in forward direction`() {
    val cursor = ConsecutiveSearchCursor(from = 0uL, to = 55uL, chunkSize = 10, direction = SearchDirection.FORWARD)

    val chunks = mutableListOf<ULongRange>()
    while (cursor.hasNext()) {
      chunks.add(cursor.next())
    }

    assertThat(chunks).containsExactly(
      0uL..9uL,
      10uL..19uL,
      20uL..29uL,
      30uL..39uL,
      40uL..49uL,
      50uL..55uL
    )
  }

  @Test
  fun `should iterate over chunks in backward direction`() {
    val cursor = ConsecutiveSearchCursor(from = 0uL, to = 55uL, chunkSize = 10, direction = SearchDirection.BACKWARD)

    val chunks = mutableListOf<ULongRange>()
    while (cursor.hasNext()) {
      chunks.add(cursor.next())
    }

    assertThat(chunks).containsExactly(
      50uL..55uL,
      40uL..49uL,
      30uL..39uL,
      20uL..29uL,
      10uL..19uL,
      0uL..9uL
    )
  }

  @Test
  fun `should throw exception when no more chunks are available`() {
    val cursor = ConsecutiveSearchCursor(from = 0uL, to = 20uL, chunkSize = 10, direction = SearchDirection.FORWARD)

    cursor.next() // 0uL..9uL
    cursor.next() // 10uL..19uL
    cursor.next() // 20uL..20uL

    val exception = assertThrows<NoSuchElementException> {
      cursor.next()
    }

    assertThat(exception).hasMessage("No more chunks available.")
  }

  @Test
  fun `should return false for hasNext when all chunks are iterated`() {
    val cursor = ConsecutiveSearchCursor(from = 0uL, to = 20uL, chunkSize = 10, direction = SearchDirection.FORWARD)

    cursor.next() // 0uL..9uL
    cursor.next() // 10uL..19uL
    cursor.next() // 20uL..20uL

    assertThat(cursor.hasNext()).isFalse
  }

  @Test
  fun `should handle single chunk range`() {
    val cursor = ConsecutiveSearchCursor(from = 0uL, to = 5uL, chunkSize = 10, direction = SearchDirection.FORWARD)

    val chunks = mutableListOf<ULongRange>()
    while (cursor.hasNext()) {
      chunks.add(cursor.next())
    }

    assertThat(chunks).containsExactly(0uL..5uL)
  }
}
