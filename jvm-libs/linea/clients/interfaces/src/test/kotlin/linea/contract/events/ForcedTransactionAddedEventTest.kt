package linea.contract.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.decodeHex
import linea.kotlin.toHexStringUInt256
import linea.kotlin.toULongFromHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ForcedTransactionAddedEventTest {
  private val forcedTransactionRollingHash = "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"
  private val rlpEncodedTx = "0x02f86d82014301843b9aca00847735940082520894" +
    "1234567890123456789012345678901234567890872386f26fc1000080c080" +
    "a01234567890123456789012345678901234567890123456789012345678901234" +
    "a01234567890123456789012345678901234567890123456789012345678901234"

  @Test
  fun `fromEthLog should map EthLog to ForcedTransactionAddedEvent correctly`() {
    /**
     * Test based on ForcedTransactionAdded event structure:
     * event ForcedTransactionAdded(
     *   uint256 indexed forcedTransactionNumber,
     *   address indexed from,
     *   uint256 blockNumberDeadline,
     *   bytes32 forcedTransactionRollingHash,
     *   bytes rlpEncodedSignedTransaction
     * );
     */
    // Calculate ABI encoding offsets
    // data layout: [blockNumberDeadline 32][forcedTransactionRollingHash 32][offset 32][length 32]
    // [rlpEncodedSignedTransaction...]
    val offset = 96 // 3 * 32 bytes (blockNumberDeadline, forcedTransactionRollingHash, offset itself)
    val rlpBytes = rlpEncodedTx.decodeHex()
    val rlpLength = rlpBytes.size
    val ethLog = EthLog(
      removed = false,
      logIndex = "0x1".toULongFromHex(),
      transactionIndex = "0x0".toULongFromHex(),
      transactionHash = "0xa1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456".decodeHex(),
      blockHash = "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef".decodeHex(),
      blockNumber = "0x100".toULongFromHex(),
      address = "0xd19d4b5d358258f05d7b411e21a1460d11b0876f".decodeHex(),
      // data = data.decodeHex(),
      (
        "0x" +
          "0000000000000000000000000000000000000000000000000000000000989680" + // blockNumberDeadline: 10000000
          forcedTransactionRollingHash + // forcedTransactionRollingHash
          offset.toULong().toHexStringUInt256(hexPrefix = false) + // offset: 96
          rlpLength.toULong().toHexStringUInt256(hexPrefix = false) + // length
          rlpEncodedTx.substring(2) // rlpEncodedSignedTransaction (without 0x)
        ).decodeHex(),
      topics = listOf(
        "0x8fbc8fbd65675eb32c567d4a559963c7d002c2be67b5b266fb13d85b4375fce5".decodeHex(), // event signature
        // forcedTransactionNumber: 100
        "0x0000000000000000000000000000000000000000000000000000000000000064".decodeHex(),
        "0x00000000000000000000000017e764ba16c95815ca06fb5d174f08d842e340df".decodeHex(), // from address
      ),
    )

    // When
    val result = ForcedTransactionAddedEvent.fromEthLog(ethLog)

    // Then
    val expectedEvent = ForcedTransactionAddedEvent(
      forcedTransactionNumber = 100UL,
      from = "0x17e764ba16c95815ca06fb5d174f08d842e340df".decodeHex(),
      blockNumberDeadline = 10000000UL,
      forcedTransactionRollingHash = forcedTransactionRollingHash.decodeHex(),
      rlpEncodedSignedTransaction = rlpEncodedTx.decodeHex(),
    )
    val expectedEthLogEvent = EthLogEvent(
      event = expectedEvent,
      log = ethLog,
    )

    assertThat(result).isEqualTo(expectedEthLogEvent)
  }

  @Test
  fun `equals should return true for identical events`() {
    val event1 = ForcedTransactionAddedEvent(
      forcedTransactionNumber = 100UL,
      from = "0x17e764ba16c95815ca06fb5d174f08d842e340df".decodeHex(),
      blockNumberDeadline = 10000000UL,
      forcedTransactionRollingHash = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd".decodeHex(),
      rlpEncodedSignedTransaction = "0xf86d".decodeHex(),
    )
    val event2 = ForcedTransactionAddedEvent(
      forcedTransactionNumber = 100UL,
      from = "0x17e764ba16c95815ca06fb5d174f08d842e340df".decodeHex(),
      blockNumberDeadline = 10000000UL,
      forcedTransactionRollingHash = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd".decodeHex(),
      rlpEncodedSignedTransaction = "0xf86d".decodeHex(),
    )

    assertThat(event1).isEqualTo(event2)
    assertThat(event1.hashCode()).isEqualTo(event2.hashCode())
  }

  @Test
  fun `equals should return false for different forcedTransactionNumber`() {
    val event1 = ForcedTransactionAddedEvent(
      forcedTransactionNumber = 100UL,
      from = "0x17e764ba16c95815ca06fb5d174f08d842e340df".decodeHex(),
      blockNumberDeadline = 10000000UL,
      forcedTransactionRollingHash = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd".decodeHex(),
      rlpEncodedSignedTransaction = "0xf86d".decodeHex(),
    )
    val event2 = event1.copy(forcedTransactionNumber = 101UL)

    assertThat(event1).isNotEqualTo(event2)
  }

  @Test
  fun `equals should return false for different from address`() {
    val event1 = ForcedTransactionAddedEvent(
      forcedTransactionNumber = 100UL,
      from = "0x17e764ba16c95815ca06fb5d174f08d842e340df".decodeHex(),
      blockNumberDeadline = 10000000UL,
      forcedTransactionRollingHash = "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd".decodeHex(),
      rlpEncodedSignedTransaction = "0xf86d".decodeHex(),
    )
    val event2 = event1.copy(from = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa".decodeHex())

    assertThat(event1).isNotEqualTo(event2)
  }

  @Test
  fun `event topic constant should match expected value`() {
    assertThat(ForcedTransactionAddedEvent.topic)
      .isEqualTo("0x8fbc8fbd65675eb32c567d4a559963c7d002c2be67b5b266fb13d85b4375fce5")
  }
}
