package linea.web3j

import linea.SearchDirection
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class SearchCursorTest {

  @Test
  fun `should calculate range chunks correctly`() {
    assertThat(rangeChunks(0uL, 50uL, 10)).containsExactly(
      0UL..9UL,
      10UL..19UL,
      20UL..29UL,
      30UL..39UL,
      40UL..49UL,
      50UL..50UL
    )

    assertThat(rangeChunks(0uL, 45uL, 10)).containsExactly(
      0UL..9UL,
      10UL..19UL,
      20UL..29UL,
      30UL..39UL,
      40UL..45UL
    )
  }

  @Test
  fun `next starts in the middle regardless of direction`() {
    assertThat(
      SearchCursor(
        from = 1uL,
        to = 100uL,
        chunkSize = 10
      ).next(searchDirection = null)
    ).isEqualTo(41UL to 50UL)
    assertThat(
      SearchCursor(
        from = 1uL,
        to = 100uL,
        chunkSize = 10
      ).next(searchDirection = SearchDirection.FORWARD)
    ).isEqualTo(41UL to 50UL)
    assertThat(
      SearchCursor(
        from = 1uL,
        to = 100uL,
        chunkSize = 10
      ).next(searchDirection = SearchDirection.BACKWARD)
    ).isEqualTo(41UL to 50UL)
  }

  @Test
  fun `next follows binary search when direction is FORWARD`() {
    val searchCursor = SearchCursor(from = 1uL, to = 100uL, chunkSize = 10)

    assertThat(searchCursor.next(SearchDirection.FORWARD)).isEqualTo(41UL to 50UL)
    assertThat(searchCursor.next(SearchDirection.FORWARD)).isEqualTo(71UL to 80UL)
    assertThat(searchCursor.next(SearchDirection.FORWARD)).isEqualTo(81UL to 90UL)
    assertThat(searchCursor.next(SearchDirection.FORWARD)).isEqualTo(91UL to 100UL)
  }

  @Test
  fun `next follows binary search when direction is BACKWARD`() {
    val searchCursor = SearchCursor(from = 1uL, to = 100uL, chunkSize = 10)

    assertThat(searchCursor.next(SearchDirection.BACKWARD)).isEqualTo(41UL to 50UL)
    assertThat(searchCursor.next(SearchDirection.BACKWARD)).isEqualTo(11UL to 20UL)
    assertThat(searchCursor.next(SearchDirection.BACKWARD)).isEqualTo(1UL to 10UL)
  }

  @Test
  fun `next follows binary search when direction is null`() {
    val searchCursor = SearchCursor(from = 1uL, to = 100uL, chunkSize = 10)

    assertThat(searchCursor.next(null)).isEqualTo(41UL to 50UL)
    assertThat(searchCursor.next(null)).isEqualTo(1UL to 10UL)
    assertThat(searchCursor.next(null)).isEqualTo(11UL to 20UL)
    assertThat(searchCursor.next(null)).isEqualTo(21UL to 30UL)
    assertThat(searchCursor.next(null)).isEqualTo(31UL to 40UL)
    assertThat(searchCursor.next(null)).isEqualTo(51UL to 60UL)
    assertThat(searchCursor.next(null)).isEqualTo(61UL to 70UL)
    assertThat(searchCursor.next(null)).isEqualTo(71UL to 80UL)
    assertThat(searchCursor.next(null)).isEqualTo(81UL to 90UL)
    assertThat(searchCursor.next(null)).isEqualTo(91UL to 100UL)
  }

  @Test
  fun `next iterates over chunks when no direction is provided`() {
    val searchCursor = SearchCursor(from = 1uL, to = 100uL, chunkSize = 10)
    val chunks = mutableListOf<Pair<ULong, ULong>>()
    var next = searchCursor.next(searchDirection = null)

    while (next != null) {
      chunks.add(next)
      next = searchCursor.next(searchDirection = null)
    }

    assertThat(chunks.sortedBy { it.first }).containsExactly(
      1UL to 10UL,
      11UL to 20UL,
      21UL to 30UL,
      31UL to 40UL,
      41UL to 50UL,
      51UL to 60UL,
      61UL to 70UL,
      71UL to 80UL,
      81UL to 90UL,
      91UL to 100UL
    )
  }

  @Test
  fun `next iterates over chunks when no direction is provided and follows direction when provided`() {
    val searchCursor = SearchCursor(from = 1uL, to = 200uL, chunkSize = 10)

    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(91UL to 100UL)

    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(1UL to 10UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.BACKWARD)).isNull()
  }

  @Test
  fun `next iterates follows direction without repeating`() {
    val searchCursor = SearchCursor(from = 1uL, to = 200uL, chunkSize = 10)

    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(91UL to 100UL)

    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(1UL to 10UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.FORWARD)).isEqualTo(101UL to 110UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.BACKWARD)).isEqualTo(51UL to 60UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(11UL to 20UL)
    // This backward is not technically valid in a typical binary search,
    // but it is a valid use case for this cursor because when searching for logs
    // the predicate can try to go back but search chunks are exhausted
    assertThat(searchCursor.next(searchDirection = SearchDirection.BACKWARD)).isNull()
  }

  @Test
  fun `next iterates follows direction without repeating - forward`() {
    val searchCursor = SearchCursor(from = 1uL, to = 200uL, chunkSize = 10)

    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(91UL to 100UL)

    assertThat(searchCursor.next(searchDirection = SearchDirection.FORWARD)).isEqualTo(141UL to 150UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.FORWARD)).isEqualTo(171UL to 180UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(151UL to 160UL)
    // This backward is not technically valid in a typical binary search,
    // but it is a valid use case for this cursor because when searching for logs
    // the predicate can try to go back but search chunks are exhausted
    assertThat(searchCursor.next(searchDirection = SearchDirection.BACKWARD)).isNull()
  }

  @Test
  fun `next iterates follows direction without repeating - forward 2`() {
    val searchCursor = SearchCursor(from = 1uL, to = 200uL, chunkSize = 10)

    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(91UL to 100UL)

    assertThat(searchCursor.next(searchDirection = SearchDirection.BACKWARD)).isEqualTo(41UL to 50UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.FORWARD)).isEqualTo(61UL to 70UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.FORWARD)).isEqualTo(71UL to 80UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(81UL to 90UL)
    assertThat(searchCursor.next(searchDirection = null)).isNull()
    // This backward is not technically valid in a typical binary search,
    // but it is a valid use case for this cursor because when searching for logs
    // the predicate can try to go back but search chunks are exauhsted
    assertThat(searchCursor.next(searchDirection = SearchDirection.BACKWARD)).isNull()
  }

  @Test
  fun `next iterates follows direction without repeating - forward 3`() {
    val searchCursor = SearchCursor(from = 0uL, to = 100uL, chunkSize = 5)
    // chunks:
    // 0..4,   5..9,   10..14, 15..19, 20..24,  25..29, 30..34, 35..39, 40..44, 45..49,
    // 50..54,
    // 55..59, 60..64, 65..69, 70..74, 75..79, 80..84, 85..89, 90..94, 95..99, 100..100
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(50UL to 54UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.BACKWARD)).isEqualTo(20UL to 24UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(0UL to 4UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(5UL to 9UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(10UL to 14UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.FORWARD)).isEqualTo(30UL to 34UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(15UL to 19UL)
    assertThat(searchCursor.next(searchDirection = SearchDirection.FORWARD)).isEqualTo(25UL to 29UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(35UL to 39UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(40UL to 44UL)
    assertThat(searchCursor.next(searchDirection = null)).isEqualTo(45UL to 49UL)
  }
}
