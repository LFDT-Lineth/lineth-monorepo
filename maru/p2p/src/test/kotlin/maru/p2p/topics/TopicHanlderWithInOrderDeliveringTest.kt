/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.topics

import maru.p2p.MaruPreparedGossipMessage
import maru.p2p.SubscriptionManager
import maru.p2p.ValidationResult
import maru.serialization.Deserializer
import org.apache.tuweni.bytes.Bytes
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import tech.pegasys.teku.networking.p2p.gossip.PreparedGossipMessage
import java.util.Optional
import java.util.concurrent.TimeUnit
import io.libp2p.core.pubsub.ValidationResult as Libp2pValidationResult

class TopicHanlderWithInOrderDeliveringTest {
  private var nextExpectedSequenceNumber = 1UL
  private val defaultQueueSize = 10
  private val defaultTimeout = 5L

  private fun createTopicHandler(
    maxQueueSize: Int = defaultQueueSize,
    onEvent: (ULong) -> SafeFuture<ValidationResult> = {
      nextExpectedSequenceNumber++
      SafeFuture.completedFuture(ValidationResult.Companion.Valid)
    },
    nextExpectedSequenceNumberProvider: (() -> ULong) = { nextExpectedSequenceNumber },
    isHandlingEnabled: () -> Boolean = { true },
  ): TopicHandlerWithInOrderDelivering<ULong> {
    val subscriptionManager = SubscriptionManager<ULong>()
    subscriptionManager.subscribe(onEvent)
    val deserializer =
      object : Deserializer<ULong> {
        override fun deserialize(bytes: ByteArray): ULong = Bytes.wrap(bytes).toLong().toULong()
      }
    val extractor = SequenceNumberExtractor<ULong> { it }
    return TopicHandlerWithInOrderDelivering(
      subscriptionManager = subscriptionManager,
      deserializer = deserializer,
      sequenceNumberExtractor = extractor,
      topicId = "test-topic",
      maxQueueSize = maxQueueSize,
      nextExpectedSequenceNumberProvider = nextExpectedSequenceNumberProvider,
      isHandlingEnabled = isHandlingEnabled,
    )
  }

  private fun createFailingDeserializerHandler(): TopicHandlerWithInOrderDelivering<ULong> {
    val subscriptionManager = SubscriptionManager<ULong>()
    subscriptionManager.subscribe { SafeFuture.completedFuture(ValidationResult.Companion.Valid) }
    val deserializer =
      object : Deserializer<ULong> {
        override fun deserialize(bytes: ByteArray): ULong = throw IllegalArgumentException("bad data")
      }
    val extractor = SequenceNumberExtractor<ULong> { it }
    return TopicHandlerWithInOrderDelivering(
      nextExpectedSequenceNumberProvider = { nextExpectedSequenceNumber },
      subscriptionManager = subscriptionManager,
      deserializer = deserializer,
      sequenceNumberExtractor = extractor,
      topicId = "test-topic",
      isHandlingEnabled = { true },
    )
  }

  private fun createMessage(seq: ULong): PreparedGossipMessage {
    val bytes = Bytes.ofUnsignedLong(seq.toLong())
    return MaruPreparedGossipMessage(
      origMessage = bytes,
      arrTimestamp = Optional.of(UInt64.valueOf(0)),
      domain = "test-domain",
      topicId = "test-topic",
    )
  }

  private fun SafeFuture<Libp2pValidationResult>.awaitResult(): Libp2pValidationResult =
    get(defaultTimeout, TimeUnit.SECONDS)

  private fun assertFutureResult(
    future: SafeFuture<Libp2pValidationResult>,
    expected: Libp2pValidationResult,
  ) {
    assertThat(future.awaitResult()).isEqualTo(expected)
  }

  private fun submitMessages(
    handler: TopicHandlerWithInOrderDelivering<ULong>,
    sequences: List<ULong>,
  ): List<SafeFuture<Libp2pValidationResult>> = sequences.map { handler.handleMessage(createMessage(it)) }

  private fun trackingEventHandler(
    processed: MutableList<ULong>,
    shouldIncrementSequence: Boolean = true,
  ): (ULong) -> SafeFuture<ValidationResult> =
    { value ->
      processed.add(value)
      val result = SafeFuture.completedFuture(ValidationResult.Companion.Valid as ValidationResult)
      if (shouldIncrementSequence) nextExpectedSequenceNumber++
      result
    }

  private fun sometimesFailingHandler(processed: MutableList<ULong>): (ULong) -> SafeFuture<ValidationResult> =
    { value ->
      processed.add(value)
      val result =
        if (value.toInt() % 2 == 1) {
          SafeFuture.failedFuture(RuntimeException("Subscriber failure"))
        } else {
          SafeFuture.completedFuture<ValidationResult>(ValidationResult.Companion.Valid)
        }
      nextExpectedSequenceNumber++
      result
    }

  private fun throwingHandler(processed: MutableList<ULong>): (ULong) -> SafeFuture<ValidationResult> =
    { value ->
      processed.add(value)
      // Simulate the handler throwing an exception directly for odd sequence numbers
      nextExpectedSequenceNumber++
      if (value.toInt() % 2 == 1) {
        throw RuntimeException("Subscriber failure for sequence $value")
      } else {
        SafeFuture.completedFuture(ValidationResult.Companion.Valid)
      }
    }

  // Test cases
  @Test
  fun `accepts and processes in-order message`() {
    val handler = createTopicHandler()

    val result1 = handler.handleMessage(createMessage(nextExpectedSequenceNumber))
    assertFutureResult(result1, Libp2pValidationResult.Valid)

    val result2 = handler.handleMessage(createMessage(nextExpectedSequenceNumber))
    assertFutureResult(result2, Libp2pValidationResult.Valid)
  }

  @Test
  fun `queues out-of-order message and processes when expected`() {
    val processed = mutableListOf<ULong>()
    val handler = createTopicHandler(onEvent = trackingEventHandler(processed))

    val future2 = handler.handleMessage(createMessage(2uL))
    assertThat(future2.isDone).isFalse

    val future1 = handler.handleMessage(createMessage(1uL))
    assertFutureResult(future1, Libp2pValidationResult.Valid)
    assertFutureResult(future2, Libp2pValidationResult.Valid)
    assertThat(processed).containsExactly(1uL, 2uL)
  }

  @Test
  fun `ignores duplicate or old messages`() {
    val processed = mutableListOf<ULong>()
    nextExpectedSequenceNumber = 10UL
    val handler = createTopicHandler(onEvent = trackingEventHandler(processed))

    val result = handler.handleMessage(createMessage(9uL))
    assertFutureResult(result, Libp2pValidationResult.Ignore)
    assertThat(processed).isEmpty()
  }

  @Test
  fun `drops the oldest message when out-of-order message arrives to a full queue`() {
    val handler = createTopicHandler()
    val futures = submitMessages(handler, (2uL..defaultQueueSize.toULong().inc()).toList())

    assertThat(futures.all { !it.isDone }).isTrue()

    val message12Future = handler.handleMessage(createMessage(12uL))
    assertThat(message12Future.isDone).isFalse() // Should be accepted and queued

    assertFutureResult(futures[0], Libp2pValidationResult.Ignore) // message 2 was dropped

    for (i in 1 until futures.size) {
      assertThat(futures[i].isDone).isFalse()
    }
  }

  @Test
  fun `returns Invalid when deserializer throws exception`() {
    val handler = createFailingDeserializerHandler()
    val result = handler.handleMessage(createMessage(1uL))
    assertFutureResult(result, Libp2pValidationResult.Invalid)
  }

  @Test
  fun `processed pending messages after the gap was filled`() {
    val processed = mutableListOf<ULong>()
    val handler = createTopicHandler(onEvent = trackingEventHandler(processed))

    // Fill the queue
    val futures = submitMessages(handler, (2uL..5uL).toList())

    // Now process the first message to drain the queue
    assertFutureResult(handler.handleMessage(createMessage(1uL)), Libp2pValidationResult.Valid)
    assertThat(futures.map { it.awaitResult() }).allMatch { it == Libp2pValidationResult.Valid }

    // Try to add another out-of-order message after draining
    val result2 = handler.handleMessage(createMessage(6uL))
    assertFutureResult(result2, Libp2pValidationResult.Valid)
    assertThat(processed).containsExactly(1uL, 2uL, 3uL, 4uL, 5uL, 6uL)
  }

  @Test
  fun `can process multiple messages with the same sequence number`() {
    val processed = mutableListOf<ULong>()
    val duplicateSequenceNumber = 3UL
    val handler =
      createTopicHandler(onEvent = { value ->
        val result =
          if (value == duplicateSequenceNumber && !processed.contains(value)) {
            nextExpectedSequenceNumber--
            SafeFuture.completedFuture(ValidationResult.Companion.Invalid("Test failure") as ValidationResult)
          } else {
            SafeFuture.completedFuture(ValidationResult.Companion.Valid as ValidationResult)
          }
        processed.add(value)
        nextExpectedSequenceNumber++
        result
      })

    val futures = submitMessages(handler, (2uL..5uL).toList())
    // 2 messages with sequence number 3
    val duplicateFuture = handler.handleMessage(createMessage(3uL))

    // Now process the first message to drain the queue
    assertFutureResult(handler.handleMessage(createMessage(1uL)), Libp2pValidationResult.Valid)
    assertThat(futures.map { it.awaitResult() }).allMatch { it == Libp2pValidationResult.Valid }

    assertFutureResult(duplicateFuture, Libp2pValidationResult.Invalid)
    assertThat(processed).containsExactly(1uL, 2uL, 3uL, 3uL, 4uL, 5uL)
  }

  @Test
  fun `should be resilient to failures - subscriber throws`() {
    val processed = mutableListOf<ULong>()
    val handler =
      createTopicHandler(onEvent = throwingHandler(processed))

    // Submit messages individually and collect their futures first
    val message2Future = handler.handleMessage(createMessage(2uL))
    val message4Future = handler.handleMessage(createMessage(4uL))
    val message3Future = handler.handleMessage(createMessage(3uL))
    val message5Future = handler.handleMessage(createMessage(5uL))
    val message1Future = handler.handleMessage(createMessage(1uL))

    // Then await the results with longer timeout
    val results =
      listOf(
        2uL to message2Future.awaitResult(),
        4uL to message4Future.awaitResult(),
        3uL to message3Future.awaitResult(),
        5uL to message5Future.awaitResult(),
        1uL to message1Future.awaitResult(),
      )
    assertThat(results).isEqualTo(
      listOf(
        2uL to Libp2pValidationResult.Valid,
        4uL to Libp2pValidationResult.Valid,
        3uL to Libp2pValidationResult.Invalid,
        5uL to Libp2pValidationResult.Invalid,
        1uL to Libp2pValidationResult.Invalid,
      ),
    )
    assertThat(processed).containsExactly(1uL, 2uL, 3uL, 4uL, 5uL)
  }

  @Test
  fun `should be resilient to failures - subscriber sometimes fails`() {
    val processed = mutableListOf<ULong>()
    val handler = createTopicHandler(onEvent = sometimesFailingHandler(processed))

    // add 1ul at to test out of order message handling
    val results =
      listOf(2uL, 4uL, 3uL, 5uL, 1uL)
        .map { seq -> seq to handler.handleMessage(createMessage(seq)) }
        .map { (messageId, future) -> messageId to future.join() }

    assertThat(results).isEqualTo(
      listOf(
        2uL to Libp2pValidationResult.Valid,
        4uL to Libp2pValidationResult.Valid,
        3uL to Libp2pValidationResult.Invalid,
        5uL to Libp2pValidationResult.Invalid,
        1uL to Libp2pValidationResult.Invalid,
      ),
    )
    assertThat(processed).containsExactly(1uL, 2uL, 3uL, 4uL, 5uL)
  }

  @Test
  fun `cleans up old messages when nextExpectedSequenceNumber advances independently`() {
    val processed = mutableListOf<ULong>()
    val handler = createTopicHandler(onEvent = trackingEventHandler(processed, shouldIncrementSequence = false))

    // Add some future messages to the queue
    val futures = submitMessages(handler, (2uL..5uL).toList())

    // Verify messages are queued (not processed yet)
    assertThat(processed).isEmpty()
    assertThat(futures.all { !it.isDone }).isTrue()

    // Now advance the external sequence number beyond some queued messages
    nextExpectedSequenceNumber = 4UL

    // Add an old message to trigger cleanup - this should be ignored
    val triggerCleanupFuture = handler.handleMessage(createMessage(1uL))
    assertFutureResult(triggerCleanupFuture, Libp2pValidationResult.Ignore)

    // Verify that message 4 from the original queue got processed
    assertFutureResult(futures[2], Libp2pValidationResult.Valid) // message 4
    assertThat(processed).containsExactly(4uL)

    // Old messages (2, 3) should have been cleaned up and marked as Ignore
    assertFutureResult(futures[0], Libp2pValidationResult.Ignore) // message 2
    assertFutureResult(futures[1], Libp2pValidationResult.Ignore) // message 3
    // Message 5 should still be pending
    assertThat(futures[3].isDone).isFalse()
  }

  @Test
  fun `maintains tail of chain when the queue is full`() {
    val processed = mutableListOf<ULong>()
    val queueSize = 5

    val handler =
      createTopicHandler(
        maxQueueSize = queueSize,
        onEvent = trackingEventHandler(processed, shouldIncrementSequence = false),
        nextExpectedSequenceNumberProvider = { nextExpectedSequenceNumber },
      )

    val futures = submitMessages(handler, (2uL..queueSize.toULong().inc()).toList())

    assertThat(processed).isEmpty()
    assertThat(futures.all { !it.isDone }).isTrue()

    // Message 7 should squeeze message 2 out of the queue
    val message7Future = handler.handleMessage(createMessage(7uL))
    assertThat(message7Future.isDone).isFalse()

    assertFutureResult(futures[0], Libp2pValidationResult.Ignore) // message 2 was dropped

    nextExpectedSequenceNumber = 5UL

    val message8Future = handler.handleMessage(createMessage(8uL))

    // Messages 3 and 4 should have been cleaned up (they were behind sequence 5)
    assertFutureResult(futures[1], Libp2pValidationResult.Ignore)
    assertFutureResult(futures[2], Libp2pValidationResult.Ignore)

    assertFutureResult(futures[3], Libp2pValidationResult.Valid) // message 5
    assertThat(processed).containsExactly(5uL)

    // Messages 6 and 7 and 8 should still be in queue (not processed yet)
    assertThat(futures[4].isDone).isFalse()
    assertThat(message7Future.isDone).isFalse()
    assertThat(message8Future.isDone).isFalse()
  }

  @Test
  fun `old messages trigger cleanup`() {
    val processed = mutableListOf<ULong>()
    val handler = createTopicHandler(onEvent = trackingEventHandler(processed, shouldIncrementSequence = false))

    // Add messages 5, 6, 7 to queue
    val future5 = handler.handleMessage(createMessage(5uL))
    val future6 = handler.handleMessage(createMessage(6uL))
    val future7 = handler.handleMessage(createMessage(7uL))

    // Advance external sequence to 6
    nextExpectedSequenceNumber = 6UL

    // Add message 3 (old) - should be ignored immediately and trigger cleanup
    val future3 = handler.handleMessage(createMessage(3uL))
    assertFutureResult(future3, Libp2pValidationResult.Ignore)

    // Message 5 should have been cleaned up
    assertFutureResult(future5, Libp2pValidationResult.Ignore)

    // The queued message 6 should now be processed
    assertFutureResult(future6, Libp2pValidationResult.Valid)
    assertThat(processed).containsExactly(6uL)

    // Message 7 should still be pending
    assertThat(future7.isDone).isFalse()
  }

  @Test
  fun `does not process messages when handling is disabled`() {
    val processed = mutableListOf<ULong>()
    val handler =
      createTopicHandler(
        onEvent = trackingEventHandler(processed),
        isHandlingEnabled = { false },
      )

    val result1 = handler.handleMessage(createMessage(1uL))
    assertThat(result1.isDone).isFalse()

    val result2 = handler.handleMessage(createMessage(2uL))
    assertThat(result2.isDone).isFalse()

    assertThat(processed).isEmpty()

    assertThat(nextExpectedSequenceNumber).isEqualTo(1uL)
  }

  @Test
  fun `processes messages when handling is re-enabled`() {
    val processed = mutableListOf<ULong>()
    var handlingEnabled = false
    val handler =
      createTopicHandler(
        onEvent = trackingEventHandler(processed),
        isHandlingEnabled = { handlingEnabled },
      )

    val result1 = handler.handleMessage(createMessage(1uL))
    assertThat(result1.isDone).isFalse()
    assertThat(processed).isEmpty()

    handlingEnabled = true

    val result2 = handler.handleMessage(createMessage(2uL))

    assertFutureResult(result1, Libp2pValidationResult.Valid)
    assertFutureResult(result2, Libp2pValidationResult.Valid)

    assertThat(processed).containsExactly(1uL, 2uL)
  }
}
