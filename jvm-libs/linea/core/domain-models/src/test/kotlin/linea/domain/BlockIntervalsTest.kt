package linea.domain

import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.Assertions.assertFalse
import org.junit.jupiter.api.Assertions.assertTrue
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class BlockIntervalsTest {

  @Test
  fun toIntervalList() {
    val blockIntervals = BlockIntervals(11u, listOf(21u, 27u, 35u))
    val expected = listOf(
      BlockIntervalData(11u, 21u),
      BlockIntervalData(22u, 27u),
      BlockIntervalData(28u, 35u),
    )
    Assertions.assertTrue(expected == blockIntervals.toIntervalList())
  }

  @Test
  fun `assertConsecutiveIntervals if intervals are not ordered throws error`() {
    val intervals = listOf(
      BlockIntervalData(0UL, 9UL),
      BlockIntervalData(20UL, 29UL),
      BlockIntervalData(10UL, 19UL),
    )

    val exception = assertThrows<IllegalArgumentException> {
      assertConsecutiveIntervals(intervals)
    }

    Assertions.assertEquals("Intervals must be sorted by startBlockNumber", exception.message)
  }

  @Test
  fun `assertConsecutiveIntervals if intervals are not consecutive throws error`() {
    val exception = assertThrows<IllegalArgumentException> {
      assertConsecutiveIntervals(
        listOf(
          BlockIntervalData(0UL, 9UL),
          BlockIntervalData(11UL, 19UL),
          BlockIntervalData(20UL, 29UL),
        ),
      )
    }

    org.assertj.core.api.Assertions.assertThat(exception.message).contains("Intervals must be consecutive")

    val exception2 = assertThrows<IllegalArgumentException> {
      assertConsecutiveIntervals(
        listOf(
          BlockIntervalData(0UL, 11UL),
          BlockIntervalData(10UL, 19UL),
          BlockIntervalData(20UL, 29UL),
        ),
      )
    }

    org.assertj.core.api.Assertions.assertThat(exception2.message).contains("Intervals must be consecutive")
  }

  @Test
  fun `contains should return true when other interval is fully contained`() {
    // Interval completely within
    assertTrue(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(20UL, 80UL)))
    // Equal intervals
    assertTrue(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(10UL, 100UL)))
    // Same start, other ends within
    assertTrue(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(10UL, 50UL)))
    // Other starts within, same end
    assertTrue(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(50UL, 100UL)))
    // Single block interval within
    assertTrue(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(50UL, 50UL)))
    // Single block at start boundary
    assertTrue(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(10UL, 10UL)))
    // Single block at end boundary
    assertTrue(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(100UL, 100UL)))
    // Single block intervals equal
    assertTrue(BlockIntervalData(50UL, 50UL).contains(BlockIntervalData(50UL, 50UL)))
  }

  @Test
  fun `contains should return false when other interval is not fully contained`() {
    // Other starts before and ends within
    assertFalse(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(5UL, 50UL)))
    // Other starts within and ends after
    assertFalse(BlockIntervalData(10UL, 100UL).contains(BlockIntervalData(50UL, 150UL)))
    // Other completely before
    assertFalse(BlockIntervalData(50UL, 100UL).contains(BlockIntervalData(10UL, 30UL)))
    // Other completely after
    assertFalse(BlockIntervalData(10UL, 50UL).contains(BlockIntervalData(60UL, 100UL)))
    // Other adjacent before
    assertFalse(BlockIntervalData(50UL, 100UL).contains(BlockIntervalData(40UL, 49UL)))
    // Other adjacent after
    assertFalse(BlockIntervalData(50UL, 100UL).contains(BlockIntervalData(101UL, 150UL)))
    // Other completely encompasses this interval
    assertFalse(BlockIntervalData(50UL, 100UL).contains(BlockIntervalData(10UL, 150UL)))
    // Other starts before and ends at same block
    assertFalse(BlockIntervalData(50UL, 100UL).contains(BlockIntervalData(40UL, 100UL)))
    // Other starts at same block and ends after
    assertFalse(BlockIntervalData(50UL, 100UL).contains(BlockIntervalData(50UL, 150UL)))
    // Other single block just before start boundary
    assertFalse(BlockIntervalData(50UL, 100UL).contains(BlockIntervalData(49UL, 49UL)))
    // Other single block just after end boundary
    assertFalse(BlockIntervalData(50UL, 100UL).contains(BlockIntervalData(101UL, 101UL)))
  }
}
