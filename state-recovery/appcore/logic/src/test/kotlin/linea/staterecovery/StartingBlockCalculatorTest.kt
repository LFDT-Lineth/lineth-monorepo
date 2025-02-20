package linea.staterecovery

import build.linea.domain.BlockInterval
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Nested
import org.junit.jupiter.api.Test

class StartingBlockCalculatorTest {
  @Nested
  inner class LookbackFetchingIntervals() {
    @Test
    fun `should return el interval only when recoveryStartBlockNumber is null`() {
      lookbackFetchingIntervals(
        headBlockNumber = 50UL,
        recoveryStartBlockNumber = null,
        lookbackWindow = 10UL
      ).also { intervals ->
        assertThat(intervals.l1Interval).isNull()
        assertThat(intervals.elInterval).isEqualTo(BlockInterval(41UL, 50UL))
      }

      // potential underflow case
      lookbackFetchingIntervals(
        headBlockNumber = 5UL,
        recoveryStartBlockNumber = null,
        lookbackWindow = 10UL
      ).also { intervals ->
        assertThat(intervals.l1Interval).isNull()
        assertThat(intervals.elInterval).isEqualTo(BlockInterval(0UL, 5UL))
      }
    }

    @Test
    fun `should return el only when headblock is before recoveryStartBlockNumber`() {
      lookbackFetchingIntervals(
        headBlockNumber = 50UL,
        recoveryStartBlockNumber = 51UL,
        lookbackWindow = 10UL
      ).also { intervals ->
        assertThat(intervals.l1Interval).isNull()
        assertThat(intervals.elInterval).isEqualTo(BlockInterval(41UL, 50UL))
      }
    }

    @Test
    fun `should return el only when recoveryStartBlockNumber is 1, starting from genesis`() {
      lookbackFetchingIntervals(
        headBlockNumber = 0UL,
        recoveryStartBlockNumber = 1UL,
        lookbackWindow = 10UL
      ).also { intervals ->
        assertThat(intervals.l1Interval).isNull()
        assertThat(intervals.elInterval).isEqualTo(BlockInterval(0UL, 0UL))
      }
    }

    @Test
    fun `should return l1 interval only when headblock - lookbackWindow is after recoveryStartBlockNumber`() {
      lookbackFetchingIntervals(
        headBlockNumber = 50UL,
        recoveryStartBlockNumber = 10UL,
        lookbackWindow = 10UL
      ).also { intervals ->
        assertThat(intervals.l1Interval).isEqualTo(BlockInterval(41UL, 50UL))
        assertThat(intervals.elInterval).isNull()
      }
    }

    @Test
    fun `should return both intervals when headblock - lookbackWindow has recoveryStartBlockNumber in the middle`() {
      lookbackFetchingIntervals(
        headBlockNumber = 50UL,
        recoveryStartBlockNumber = 45UL,
        lookbackWindow = 10UL
      ).also { intervals ->
        assertThat(intervals.l1Interval).isEqualTo(BlockInterval(45UL, 50UL))
        assertThat(intervals.elInterval).isEqualTo(BlockInterval(41UL, 44UL))
      }
    }
  }

  @Nested
  inner class StartBlockToFetchFromL1 {
    @Test
    fun `should return headBlockNumber + 1UL when recoveryStartBlockNumber is null`() {
      startBlockToFetchFromL1(
        headBlockNumber = 500UL,
        recoveryStartBlockNumber = null,
        lookbackWindow = 256UL
      ).also { result ->
        // Then
        assertThat(result).isEqualTo(501UL)
      }

      startBlockToFetchFromL1(
        headBlockNumber = 200UL,
        recoveryStartBlockNumber = null,
        lookbackWindow = 256UL
      ).also { result ->
        // Then
        assertThat(result).isEqualTo(201UL)
      }
    }

    @Test
    fun `should return headblock - lookBackWindow when recoveryStartBlockNumber is before than lookBackWindow`() {
      startBlockToFetchFromL1(
        headBlockNumber = 500UL,
        recoveryStartBlockNumber = 250UL,
        lookbackWindow = 100UL
      ).also { result ->
        // Then
        assertThat(result).isEqualTo(400UL)
      }
    }

    @Test
    fun `should return recoveryStartBlockNumber when headblock - recoveryStartBlockNumber less that lookBackWindow`() {
      startBlockToFetchFromL1(
        headBlockNumber = 500UL,
        recoveryStartBlockNumber = 450UL,
        lookbackWindow = 100UL
      ).also { result ->
        // Then
        assertThat(result).isEqualTo(450UL)
      }

      // potential underflow case
      startBlockToFetchFromL1(
        headBlockNumber = 50UL,
        recoveryStartBlockNumber = 45UL,
        lookbackWindow = 100UL
      ).also { result ->
        // Then
        assertThat(result).isEqualTo(45UL)
      }
    }
  }
}
