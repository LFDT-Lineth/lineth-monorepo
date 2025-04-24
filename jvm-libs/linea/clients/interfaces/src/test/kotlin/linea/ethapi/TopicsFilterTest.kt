package linea.ethapi

import linea.kotlin.decodeHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class TopicsFilterTest {

  @Test
  fun `should return true when all topics match`() {
    val logTopics = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )
    val topicsFilter = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )

    val result = FakeEthApiClient.matchesTopicFilter(logTopics, topicsFilter)

    assertThat(result).isTrue
  }

  @Test
  fun `should return true when all initial topics match and remaining are not defined`() {
    val logTopics = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex(),
      "0x3333333333333333333333333333333333333333333333333333333333333333".decodeHex()
    )
    val topicsFilter = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )

    val result = FakeEthApiClient.matchesTopicFilter(logTopics, topicsFilter)

    assertThat(result).isTrue
  }

  @Test
  fun `should return true when topics filter contains null (wildcard)`() {
    val logTopics = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )
    val topicsFilter = listOf(
      null,
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )

    val result = FakeEthApiClient.matchesTopicFilter(logTopics, topicsFilter)

    assertThat(result).isTrue
  }

  @Test
  fun `should return false when topics do not match`() {
    val logTopics = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )
    val topicsFilter = listOf(
      "0x3333333333333333333333333333333333333333333333333333333333333333".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )

    val result = FakeEthApiClient.matchesTopicFilter(logTopics, topicsFilter)

    assertThat(result).isFalse
  }

  @Test
  fun `should return true when topics filter is empty`() {
    val logTopics = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )
    val topicsFilter = emptyList<ByteArray?>()

    val result = FakeEthApiClient.matchesTopicFilter(logTopics, topicsFilter)

    assertThat(result).isTrue
  }

  @Test
  fun `should return false when log topics are fewer than topics filter`() {
    val logTopics = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex()
    )
    val topicsFilter = listOf(
      "0x1111111111111111111111111111111111111111111111111111111111111111".decodeHex(),
      "0x2222222222222222222222222222222222222222222222222222222222222222".decodeHex()
    )

    val result = FakeEthApiClient.matchesTopicFilter(logTopics, topicsFilter)

    assertThat(result).isFalse
  }
}
