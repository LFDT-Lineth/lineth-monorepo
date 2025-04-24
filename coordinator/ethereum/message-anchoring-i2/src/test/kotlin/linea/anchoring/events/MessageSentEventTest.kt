package linea.anchoring.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.decodeHex
import linea.kotlin.toULongFromHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class MessageSentEventTest {

  @Test
  fun `fromEthLog should map EthLog to MessageSentEvent correctly`() {
    val ethLog = EthLog(
      removed = false,
      logIndex = "0x1".toULongFromHex(),
      transactionIndex = "0x0".toULongFromHex(),
      transactionHash = "0x2d408675b46835a04ba632ac437ca9b9ca41b834609b7453630fe594ba658b4c".decodeHex(),
      blockHash = "0x4d63489ac2faee706cca0f078f23973facc42a87dc75cfdf6fae5ac2d8c9b243".decodeHex(),
      blockNumber = "0x1109669".toULongFromHex(),
      address = "0x508ca82df566dcd1b0de8296e70a96332cd644ec".decodeHex(),
      data = (
        "0x" +
          "00000000000000000000000000000000000000000000000000000000000003e8" + // fee
          "00000000000000000000000000000000000000000000000000000000000007d0" + // value
          "0000000000000000000000000000000000000000000000000000000000000000" + // padding
          "0000000000000000000000000000000000000000000000000000000000002710" + // nonce
          "deadbeef"
        ).decodeHex(), // calldata
      topics = listOf(
        "0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c".decodeHex(),
        "0x0000000000000000000000001234567890abcdef1234567890abcdef12345678".decodeHex(), // from
        "0x000000000000000000000000abcdef1234567890abcdef1234567890abcdef12".decodeHex(), // to
        "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd".decodeHex() // messageHash
      )
    )

    // When
    val result = MessageSentEvent.fromEthLog(ethLog)

    // Then
    val expectedEvent = MessageSentEvent(
      from = "0x1234567890abcdef1234567890abcdef12345678".decodeHex(),
      to = "0xabcdef1234567890abcdef1234567890abcdef12".decodeHex(),
      fee = 1000uL,
      value = 2000uL,
      messageNumber = 10000uL,
      calldata = "deadbeef".decodeHex(),
      messageHash = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd".decodeHex()
    )
    val expectedEthLogEvent = EthLogEvent(
      event = expectedEvent,
      log = ethLog
    )

    assertThat(result).isEqualTo(expectedEthLogEvent)
  }

  private val eventTemplate = MessageSentEvent(
    from = "0x000000000000000000000000000000000000000a".decodeHex(),
    to = "0x000000000000000000000000000000000000000b".decodeHex(),
    fee = 0uL,
    value = 0uL,
    messageNumber = 1uL,
    calldata = ByteArray(0),
    messageHash = "0x0000000000000000000000000000000000000000".decodeHex()
  )

  @Test
  fun `compareTo should return negative when messageNumber is smaller`() {
    val event1 = eventTemplate.copy(messageNumber = 1uL)
    val event2 = eventTemplate.copy(messageNumber = 2uL)

    assertThat(event1.compareTo(event2)).isLessThan(0)
  }

  @Test
  fun `compareTo should return positive when messageNumber is larger`() {
    val event1 = eventTemplate.copy(messageNumber = 2uL)
    val event2 = eventTemplate.copy(messageNumber = 1uL)

    assertThat(event1.compareTo(event2)).isGreaterThan(0)
  }

  @Test
  fun `compareTo should return zero when messageNumber is equal`() {
    val event1 = eventTemplate.copy(messageNumber = 2uL)
    val event2 = eventTemplate.copy(messageNumber = 2uL)

    assertThat(event1.compareTo(event2)).isEqualTo(0)
  }
}
