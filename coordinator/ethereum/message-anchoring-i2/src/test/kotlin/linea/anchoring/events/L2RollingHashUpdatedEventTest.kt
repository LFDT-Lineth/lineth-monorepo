package linea.anchoring.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.decodeHex
import linea.kotlin.toULongFromHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class L2RollingHashUpdatedEventTest {
  @Test
  fun `fromEthLog should map EthLog to L2RollingHashUpdatedEvent correctly`() {
    // Given
    val ethLog = EthLog(
      removed = false,
      logIndex = "0x1".toULongFromHex(),
      transactionIndex = "0x0".toULongFromHex(),
      transactionHash = "0x2d408675b46835a04ba632ac437ca9b9ca41b834609b7453630fe594ba658b4c".decodeHex(),
      blockHash = "0x4d63489ac2faee706cca0f078f23973facc42a87dc75cfdf6fae5ac2d8c9b243".decodeHex(),
      blockNumber = "0x1109669".toULongFromHex(),
      address = "0x508ca82df566dcd1b0de8296e70a96332cd644ec".decodeHex(),
      data = "0x".decodeHex(),
      topics = listOf(
        "0x99b65a4301b38c09fb6a5f27052d73e8372bbe8f6779d678bfe8a41b66cce7ac".decodeHex(),
        "0x00000000000000000000000000000000000000000000000000000000000b415e".decodeHex(),
        "0x3444eb64c4a09587c01e9102c567e34f9fc9a6a367c2c5abad5a57dbf1df98de".decodeHex()
      )
    )

    // When
    val result = L2RollingHashUpdatedEvent.fromEthLog(ethLog)

    // Then
    val expectedEvent = L2RollingHashUpdatedEvent(
      messageNumber = 0xB415E.toULong(),
      rollingHash = "0x3444eb64c4a09587c01e9102c567e34f9fc9a6a367c2c5abad5a57dbf1df98de".decodeHex()
    )
    val expectedEthLogEvent = EthLogEvent(
      event = expectedEvent,
      log = ethLog
    )

    assertThat(result).isEqualTo(expectedEthLogEvent)
  }
}
