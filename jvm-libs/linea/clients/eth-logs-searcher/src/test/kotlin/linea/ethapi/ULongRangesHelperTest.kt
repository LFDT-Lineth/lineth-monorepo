package linea.ethapi

import linea.kotlin.intersection
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

internal fun generateEffectiveIntervals(
  blocksWithLogs: List<ULongRange>,
  filterFromBlock: ULong,
  filterToBlock: ULong
): List<ULongRange> {
  // if blocksWithLogs is [10..19, 25..29, 40..49] and filter.fromBlock=15 and filter.toBlock=45
  // then we will return logs for blocks [15..19, 25..29, 40..45]
  val fromToRange = filterFromBlock..filterToBlock

  return blocksWithLogs
    .map { it.intersection(fromToRange) }
    .filter { !it.isEmpty() }
    .map { range -> (range.first.coerceAtLeast(filterFromBlock))..(range.last.coerceAtMost(filterToBlock)) }
}

class ULongRangesHelperTest {

  @Test
  fun `generateEffectiveIntervals returns effective intervals for given blocks with logs and filter`() {
    val blocksWithLogs = listOf(10UL..19UL, 40UL..49UL, 60UL..69UL)

    // before range
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 0UL,
      filterToBlock = 9UL
    )
      .also {
        assertThat(it).isEmpty()
      }

    // intersect 1st half of range
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 0UL,
      filterToBlock = 15UL
    )
      .also {
        assertThat(it).containsExactly(10UL..15UL)
      }
    // intersect 2st half of range
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 15UL,
      filterToBlock = 25UL
    )
      .also {
        assertThat(it).containsExactly(15UL..19UL)
      }

    // overlapping single range
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 0UL,
      filterToBlock = 25UL
    )
      .also {
        assertThat(it).containsExactly(10UL..19UL)
      }

    // within single range
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 42UL,
      filterToBlock = 45UL
    )
      .also {
        assertThat(it).containsExactly(42UL..45UL)
      }

    // after the range
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 50UL,
      filterToBlock = 55UL
    )
      .also {
        assertThat(it).isEmpty()
      }

    // overlapping all ranges
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 0UL,
      filterToBlock = 100UL
    )
      .also {
        assertThat(it).containsExactly(10UL..19UL, 40UL..49UL, 60UL..69UL)
      }

    // overlapping middle ranges and intersect 1st half and 2nd of edge ranges
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 15UL,
      filterToBlock = 65UL
    )
      .also {
        assertThat(it).containsExactly(15UL..19UL, 40UL..49UL, 60UL..65UL)
      }

    // intersect 1st half and 2nd of ranges
    generateEffectiveIntervals(
      blocksWithLogs = blocksWithLogs,
      filterFromBlock = 15UL,
      filterToBlock = 45UL
    )
      .also {
        assertThat(it).containsExactly(15UL..19UL, 40UL..45UL)
      }
  }
}
