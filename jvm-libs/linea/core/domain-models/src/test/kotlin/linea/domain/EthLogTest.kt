package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class EthLogTest {
  private val ethLog1 = EthLog(
    removed = false,
    logIndex = 2UL,
    transactionIndex = 1UL,
    transactionHash = byteArrayOf(1, 2, 3),
    blockHash = byteArrayOf(4, 5, 6),
    blockNumber = 123UL,
    address = byteArrayOf(7, 8, 9),
    data = byteArrayOf(10, 11, 12),
    topics = listOf(byteArrayOf(13, 14), byteArrayOf(15, 16))
  )
  private val ethLog2 = ethLog1.copy(blockNumber = 124UL)
  private val ethLog3 = ethLog1.copy(blockNumber = 123UL, logIndex = 3UL)
  private val ethLog4 = ethLog1.copy(blockNumber = 123UL, logIndex = 2UL)

  @Test
  fun `equals should return true for identical objects`() {
    val ethLog2 = ethLog1.copy(topics = listOf(byteArrayOf(13, 14), byteArrayOf(15, 16)))

    assertThat(ethLog1).isEqualTo(ethLog2)
  }

  @Test
  fun `equals should return false for different objects`() {
    val ethLog2 = ethLog1.copy(topics = listOf(byteArrayOf(13, 99), byteArrayOf(15, 16)))

    assertThat(ethLog1).isNotEqualTo(ethLog2)
  }

  @Test
  fun `hashCode should return the same value for identical objects`() {
    val ethLog2 = ethLog1.copy(topics = listOf(byteArrayOf(13, 14), byteArrayOf(15, 16)))

    assertThat(ethLog1.hashCode()).isEqualTo(ethLog2.hashCode())
  }

  @Test
  fun `hashCode should return different values for different objects`() {
    val ethLog2 = ethLog1.copy(topics = listOf(byteArrayOf(13, 99), byteArrayOf(15, 16)))

    assertThat(ethLog1.hashCode()).isNotEqualTo(ethLog2.hashCode())
  }

  @Test
  fun `compareTo should return negative when blockNumber is smaller`() {
    val event1 = EthLogEvent("event1", ethLog1.copy(blockNumber = 123UL))
    val event2 = EthLogEvent("event2", ethLog1.copy(blockNumber = 124UL))

    assertThat(event1.compareTo(event2)).isLessThan(0)
  }

  @Test
  fun `compareTo should return positive when blockNumber is larger`() {
    val event1 = EthLogEvent("event1", ethLog1.copy(blockNumber = 124UL))
    val event2 = EthLogEvent("event2", ethLog1.copy(blockNumber = 123UL))

    assertThat(event1.compareTo(event2)).isGreaterThan(0)
  }

  @Test
  fun `compareTo should return negative when blockNumber is equal but logIndex is smaller`() {
    val event1 = EthLogEvent("event1", ethLog1.copy(blockNumber = 123UL, logIndex = 2UL))
    val event2 = EthLogEvent("event2", ethLog1.copy(blockNumber = 123UL, logIndex = 3UL))

    assertThat(event1.compareTo(event2)).isLessThan(0)
  }

  @Test
  fun `compareTo should return positive when blockNumber is equal but logIndex is larger`() {
    val event1 = EthLogEvent("event1", ethLog1.copy(blockNumber = 123UL, logIndex = 3UL))
    val event2 = EthLogEvent("event2", ethLog1.copy(blockNumber = 123UL, logIndex = 2UL))

    assertThat(event1.compareTo(event2)).isGreaterThan(0)
  }

  @Test
  fun `compareTo should return zero when blockNumber and logIndex are equal`() {
    val event1 = EthLogEvent("event1", ethLog1.copy(blockNumber = 123UL, logIndex = 3UL))
    val event2 = EthLogEvent("event2", ethLog1.copy(blockNumber = 123UL, logIndex = 3UL))

    assertThat(event1.compareTo(event2)).isEqualTo(0)
  }
}
