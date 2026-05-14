/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.topics

import maru.p2p.LINEA_DOMAIN
import maru.p2p.MaruPreparedGossipMessage
import maru.p2p.SubscriptionManager
import maru.p2p.ValidationResult
import maru.serialization.Deserializer
import maru.serialization.MAX_MESSAGE_SIZE
import org.apache.tuweni.bytes.Bytes
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import tech.pegasys.teku.networking.p2p.gossip.PreparedGossipMessage
import java.util.Optional
import io.libp2p.core.pubsub.ValidationResult as Libp2pValidationResult

class ImmediateTopicHandlerTest {
  private val mockSubscriptionManager = mock<SubscriptionManager<String>>()
  private val topicId = "test-topic"
  private lateinit var deserializer: TestDeserializer
  private lateinit var handler: ImmediateTopicHandler<String>

  @BeforeEach
  fun setUp() {
    deserializer = TestDeserializer()
    handler = ImmediateTopicHandler(
      subscriptionManager = mockSubscriptionManager,
      deserializer = deserializer,
      topicId = topicId,
    )
  }

  private class TestDeserializer : Deserializer<String> {
    private val values = mutableMapOf<ByteArray, String>()
    private var exceptionToThrow: RuntimeException? = null

    fun configureResult(
      input: ByteArray,
      output: String,
    ) {
      values[input] = output
    }

    fun configureToThrow(exception: RuntimeException) {
      exceptionToThrow = exception
    }

    override fun deserialize(bytes: ByteArray): String {
      exceptionToThrow?.let { throw it }
      return values.entries.firstOrNull { it.key.contentEquals(bytes) }?.value
        ?: throw IllegalStateException("No configured result for input")
    }
  }

  @Test
  fun `prepareMessage creates MaruPreparedGossipMessage with correct parameters`() {
    val payload = Bytes.fromHexString("0x1234567890abcdef")
    val arrivalTimestamp = Optional.of(UInt64.valueOf(12345L))

    val result = handler.prepareMessage(payload, arrivalTimestamp)

    assertThat(result).isInstanceOf(MaruPreparedGossipMessage::class.java)
    val maruMessage = result as MaruPreparedGossipMessage
    assertThat(maruMessage.originalMessage).isEqualTo(payload)
    assertThat(maruMessage.arrivalTimestamp).isEqualTo(arrivalTimestamp)
    // Verify domain and topicId are embedded in messageId
    val expectedMessageId = handler.prepareMessage(payload, arrivalTimestamp).messageId
    assertThat(maruMessage.messageId).isEqualTo(expectedMessageId)
  }

  @Test
  fun `handleMessage successfully processes valid message`() {
    val payload = Bytes.fromHexString("0x1234")
    val deserializedMessage = "test-message"
    val message = createPreparedMessage(payload)

    deserializer.configureResult(payload.toArray(), deserializedMessage)
    whenever(mockSubscriptionManager.handleEvent(deserializedMessage))
      .thenReturn(SafeFuture.completedFuture(ValidationResult.Companion.Valid))

    val result = handler.handleMessage(message).join()

    assertThat(result).isEqualTo(Libp2pValidationResult.Valid)
    verify(mockSubscriptionManager).handleEvent(deserializedMessage)
  }

  @Test
  fun `handleMessage returns Invalid when subscription manager returns Invalid`() {
    val payload = Bytes.fromHexString("0x1234")
    val deserializedMessage = "test-message"
    val message = createPreparedMessage(payload)

    deserializer.configureResult(payload.toArray(), deserializedMessage)
    whenever(mockSubscriptionManager.handleEvent(deserializedMessage))
      .thenReturn(SafeFuture.completedFuture(ValidationResult.Companion.Invalid("test error")))

    val result = handler.handleMessage(message).join()

    assertThat(result).isEqualTo(Libp2pValidationResult.Invalid)
  }

  @Test
  fun `handleMessage returns Ignore when subscription manager returns Ignore`() {
    val payload = Bytes.fromHexString("0x1234")
    val deserializedMessage = "test-message"
    val message = createPreparedMessage(payload)

    deserializer.configureResult(payload.toArray(), deserializedMessage)
    whenever(mockSubscriptionManager.handleEvent(deserializedMessage))
      .thenReturn(SafeFuture.completedFuture(ValidationResult.Companion.Ignore("test ignore")))

    val result = handler.handleMessage(message).join()

    assertThat(result).isEqualTo(Libp2pValidationResult.Ignore)
  }

  @Test
  fun `handleMessage returns Invalid when deserializer throws exception`() {
    val payload = Bytes.fromHexString("0x1234")
    val message = createPreparedMessage(payload)

    deserializer.configureToThrow(RuntimeException("Deserialization failed"))

    val result = handler.handleMessage(message).join()

    assertThat(result).isEqualTo(Libp2pValidationResult.Invalid)
    verify(mockSubscriptionManager, never()).handleEvent(any())
  }

  @Test
  fun `handleMessage returns Invalid when subscription manager future fails`() {
    val payload = Bytes.fromHexString("0x1234")
    val deserializedMessage = "test-message"
    val message = createPreparedMessage(payload)

    deserializer.configureResult(payload.toArray(), deserializedMessage)
    whenever(mockSubscriptionManager.handleEvent(deserializedMessage))
      .thenReturn(SafeFuture.failedFuture(RuntimeException("Subscription manager error")))

    val result = handler.handleMessage(message).join()

    assertThat(result).isEqualTo(Libp2pValidationResult.Invalid)
  }

  @Test
  fun `handleMessage returns Invalid when subscription manager handleEvent throws exception`() {
    val payload = Bytes.fromHexString("0x1234")
    val deserializedMessage = "test-message"
    val message = createPreparedMessage(payload)

    deserializer.configureResult(payload.toArray(), deserializedMessage)
    whenever(mockSubscriptionManager.handleEvent(deserializedMessage))
      .thenThrow(RuntimeException("Direct exception from handleEvent"))

    val result = handler.handleMessage(message).join()

    assertThat(result).isEqualTo(Libp2pValidationResult.Invalid)
  }

  @Test
  fun `getMaxMessageSize returns MAX_MESSAGE_SIZE`() {
    assertThat(handler.getMaxMessageSize()).isEqualTo(MAX_MESSAGE_SIZE)
  }

  @Test
  fun `handleMessage processes multiple messages independently`() {
    val payload1 = Bytes.fromHexString("0x1234")
    val payload2 = Bytes.fromHexString("0x5678")
    val deserializedMessage1 = "message-1"
    val deserializedMessage2 = "message-2"
    val message1 = createPreparedMessage(payload1)
    val message2 = createPreparedMessage(payload2)

    deserializer.configureResult(payload1.toArray(), deserializedMessage1)
    deserializer.configureResult(payload2.toArray(), deserializedMessage2)
    whenever(mockSubscriptionManager.handleEvent(deserializedMessage1))
      .thenReturn(SafeFuture.completedFuture(ValidationResult.Companion.Valid))
    whenever(mockSubscriptionManager.handleEvent(deserializedMessage2))
      .thenReturn(SafeFuture.completedFuture(ValidationResult.Companion.Ignore("ignore this")))

    val result1 = handler.handleMessage(message1).join()
    val result2 = handler.handleMessage(message2).join()

    assertThat(result1).isEqualTo(Libp2pValidationResult.Valid)
    assertThat(result2).isEqualTo(Libp2pValidationResult.Ignore)
    verify(mockSubscriptionManager).handleEvent(deserializedMessage1)
    verify(mockSubscriptionManager).handleEvent(deserializedMessage2)
  }

  private fun createPreparedMessage(payload: Bytes): PreparedGossipMessage =
    MaruPreparedGossipMessage(
      origMessage = payload,
      arrTimestamp = Optional.of(UInt64.valueOf(0)),
      domain = LINEA_DOMAIN,
      topicId = topicId,
    )
}
