package net.consensys.zkevm.ethereum.coordination.common

import net.consensys.zkevm.ethereum.coordination.MaxLongTracker
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class MaxLongTrackerTest {
  @Test
  fun `long tracker tracks the long value`() {
    val maxLongTracker =
      object : MaxLongTracker<String>(12L) {
        override fun convertToLong(trackable: String): Long {
          return trackable.toLong()
        }
      }

    maxLongTracker.invoke("10")
    assertThat(maxLongTracker.get()).isEqualTo(12L)
    maxLongTracker.invoke("13")
    assertThat(maxLongTracker.get()).isEqualTo(13L)
    maxLongTracker.invoke("1")
    assertThat(maxLongTracker.get()).isEqualTo(13L)
  }
}
