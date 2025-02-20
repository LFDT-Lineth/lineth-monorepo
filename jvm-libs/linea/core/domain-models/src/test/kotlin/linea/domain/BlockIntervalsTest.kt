package linea.domain

import org.junit.jupiter.api.Assertions
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows

class BlockIntervalsTest {

  @Test
  fun toIntervalList() {
    val blockIntervals = BlockIntervals(11u, listOf(21u, 27u, 35u))
    val expected = listOf(
      BlockIntervalData(11u, 21u),
      BlockIntervalData(22u, 27u),
      BlockIntervalData(28u, 35u)
    )
    Assertions.assertTrue(expected == blockIntervals.toIntervalList())
  }

  @Test
  fun `assertConsecutiveIntervals if intervals are not ordered throws error`() {
    val intervals = listOf(
      BlockIntervalData(0UL, 9UL),
      BlockIntervalData(20UL, 29UL),
      BlockIntervalData(10UL, 19UL)
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
          BlockIntervalData(20UL, 29UL)
        )
      )
    }

    org.assertj.core.api.Assertions.assertThat(exception.message).contains("Intervals must be consecutive")

    val exception2 = assertThrows<IllegalArgumentException> {
      assertConsecutiveIntervals(
        listOf(
          BlockIntervalData(0UL, 11UL),
          BlockIntervalData(10UL, 19UL),
          BlockIntervalData(20UL, 29UL)
        )
      )
    }

    org.assertj.core.api.Assertions.assertThat(exception2.message).contains("Intervals must be consecutive")
  }
}
