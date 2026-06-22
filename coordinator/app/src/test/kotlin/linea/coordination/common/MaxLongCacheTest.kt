package linea.coordination.common

import linea.coordination.MaxLongCache
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class MaxLongCacheTest {
  @Test
  fun `accept saves the max seen end block number`() {
    val initialProvenBlockNumber = 3L
    val maxLongCache = MaxLongCache(initialProvenBlockNumber)

    assertThat(maxLongCache.get()).isEqualTo(initialProvenBlockNumber)

    val expectedMaxProvenBlockNumber = 10L
    maxLongCache.accept(expectedMaxProvenBlockNumber)
    maxLongCache.accept(5L)

    assertThat(maxLongCache.get()).isEqualTo(expectedMaxProvenBlockNumber)
  }
}
