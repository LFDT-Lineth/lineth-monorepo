package linea.contract.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.decodeHex
import linea.kotlin.toULongFromHex
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.Test

class L1RollingHashUpdatedEventTest {

  @Test
  fun `fromEthLog should map EthLog to L1RollingHashUpdatedEvent correctly`() {
    // Given
    val ethLog = EthLog(
      removed = false,
      logIndex = "0x153".toULongFromHex(),
      transactionIndex = "0xa3".toULongFromHex(),
      transactionHash = "0x2fcc21abd268b037d27e612d8ad4b970343540c61fc3df40bf3f9605b6a2ea6a".decodeHex(),
      blockHash = "0x1147f0700828a800226e11391b4e7cc68cc72db608f82b1e07316f6d32b43b60".decodeHex(),
      blockNumber = "0x15335b9".toULongFromHex(),
      address = "0xd19d4b5d358258f05d7b411e21a1460d11b0876f".decodeHex(),
      data = "0x".decodeHex(),
      topics = listOf(
        "0xea3b023b4c8680d4b4824f0143132c95476359a2bb70a81d6c5a36f6918f6339".decodeHex(),
        "0x00000000000000000000000000000000000000000000000000000000000b415a".decodeHex(),
        "0x7abd5eea8cbb46bba0aa83369dcc0d9b18931a825b73f45d98da586070eafa8b".decodeHex(),
        "0x24dca2d33621322ef7c85d7cea38b673c06cbb86a7f15c8aa5f658485f932fd0".decodeHex()
      )
    )

    // When
    val result = L1RollingHashUpdatedEvent.fromEthLog(ethLog)

    // Then
    val expectedEvent = L1RollingHashUpdatedEvent(
      messageNumber = 0xB415A.toULong(),
      rollingHash = "0x7abd5eea8cbb46bba0aa83369dcc0d9b18931a825b73f45d98da586070eafa8b".decodeHex(),
      messageHash = "0x24dca2d33621322ef7c85d7cea38b673c06cbb86a7f15c8aa5f658485f932fd0".decodeHex()
    )
    val expectedEthLogEvent = EthLogEvent(
      event = expectedEvent,
      log = ethLog
    )

    Assertions.assertThat(result).isEqualTo(expectedEthLogEvent)
  }
}
