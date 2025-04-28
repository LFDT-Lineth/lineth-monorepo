package linea.anchoring.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.decodeHex
import linea.kotlin.toBigInteger
import linea.kotlin.toULongFromHex
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.math.BigInteger

class MessageSentEventTest {

  @Test
  fun `fromEthLog should map EthLog to MessageSentEvent correctly 2`() {
    /**
     * curl -s -H 'content-type:application/json' --data '{"jsonrpc":"2.0","id":"53","method":"eth_getLogs","params":[{"address":["0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5"], "topics":["0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c",null,null,"0x6011E7F01AAE0E2CD2650CBE229A330F93821D5ED8A2E8830D1E64BA3C76CC3F"], "fromBlock":"earliest","toBlock":"latest"}]}' $URL_SEPOLIA  | jq '.result'
     * [
     *   {
     *     "removed": false,
     *     "logIndex": "0x8c",
     *     "transactionIndex": "0x36",
     *     "transactionHash": "0x7fd853e580f967cf9004a019bcce26b8d4d2b880ef1106f5af29e2e23695e9ec",
     *     "blockHash": "0x8c08c5c7f1d8a256a392f5d261b9e4ba39337903646ba009f16a0b2029447c20",
     *     "blockNumber": "0x7cec04",
     *     "address": "0xb218f8a4bc926cf1ca7b3423c154a0d627bdb7e5",
     *     "data": "0x000000000000000000000000000000000000000000000000002386f26fc100000000000000000000000000000000000000000000000000000de0b6b3a764000000000000000000000000000000000000000000000000000000000000000140430000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000001d54686973206973206a75737420666f7220796f7520676f6f6420736972000000",
     *     "topics": [
     *       "0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c",
     *       "0x00000000000000000000000017e764ba16c95815ca06fb5d174f08d842e340df",
     *       "0x0000000000000000000000007a98052d4be677df72ede5a3f2829c893d10388d",
     *       "0x6011e7f01aae0e2cd2650cbe229a330f93821d5ed8a2e8830d1e64ba3c76cc3f"
     *     ]
     *   }
     * ]
     */
    val callData = "0000000000000000000000000000000000000000000000000000000000000080" +
      "000000000000000000000000000000000000000000000000000000000000001d" +
      "54686973206973206a75737420666f7220796f7520676f6f6420736972000000"
    val ethLog = EthLog(
      removed = false,
      logIndex = "0x1".toULongFromHex(),
      transactionIndex = "0x0".toULongFromHex(),
      transactionHash = "0x7fd853e580f967cf9004a019bcce26b8d4d2b880ef1106f5af29e2e23695e9ec".decodeHex(),
      blockHash = "0x8c08c5c7f1d8a256a392f5d261b9e4ba39337903646ba009f16a0b2029447c20".decodeHex(),
      blockNumber = "0x7cec04".toULongFromHex(),
      address = "0xb218f8a4bc926cf1ca7b3423c154a0d627bdb7e5".decodeHex(),
      data = (
        "0x" +
          "000000000000000000000000000000000000000000000000002386f26fc10000" + // fee
          "0000000000000000000000000000000000000000000000000de0b6b3a7640000" + // value
          "0000000000000000000000000000000000000000000000000000000000014043" + // messageNumber
          // calldata
          callData
        ).decodeHex(),
      topics = listOf(
        "0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c".decodeHex(),
        "0x00000000000000000000000017e764ba16c95815ca06fb5d174f08d842e340df".decodeHex(), // from
        "0x0000000000000000000000007a98052d4be677df72ede5a3f2829c893d10388d".decodeHex(), // to
        "0x6011e7f01aae0e2cd2650cbe229a330f93821d5ed8a2e8830d1e64ba3c76cc3f".decodeHex() // messageHash
      )
    )

    // When
    val result = MessageSentEvent.fromEthLog(ethLog)

    // Then
    val expectedEvent = MessageSentEvent(
      messageNumber = 81987UL,
      from = "0x17e764ba16c95815ca06fb5d174f08d842e340df".decodeHex(),
      to = "0x7a98052d4be677df72ede5a3f2829c893d10388d".decodeHex(),
      fee = BigInteger("2386f26fc10000", 16),
      value = BigInteger("de0b6b3a7640000", 16),
      calldata = "0x$callData".decodeHex(),
      messageHash = "0x6011e7f01aae0e2cd2650cbe229a330f93821d5ed8a2e8830d1e64ba3c76cc3f".decodeHex()
    )
    val expectedEthLogEvent = EthLogEvent(
      event = expectedEvent,
      log = ethLog
    )

    assertThat(result).isEqualTo(expectedEthLogEvent)
  }

  private val eventTemplate = MessageSentEvent(
    messageNumber = 1uL,
    from = "0x000000000000000000000000000000000000000a".decodeHex(),
    to = "0x000000000000000000000000000000000000000b".decodeHex(),
    fee = 0uL.toBigInteger(),
    value = 0uL.toBigInteger(),
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
