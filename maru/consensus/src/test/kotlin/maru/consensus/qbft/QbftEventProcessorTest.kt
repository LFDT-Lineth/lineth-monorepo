/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.consensus.common.bft.BftEventQueue
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.events.BlockTimerExpiry
import org.hyperledger.besu.consensus.common.bft.events.RoundExpiry
import org.junit.jupiter.api.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.RejectedExecutionException
import java.util.concurrent.TimeUnit

class QbftEventProcessorTest {
  @Test
  fun `each run has its own completion and finishes the current event before stopping`() {
    val queue = BftEventQueue(1000)
    var eventStarted = CountDownLatch(1)
    var releaseEvent = CountDownLatch(1)
    val handler = FakeQbftEventHandler {
      eventStarted.countDown()
      check(releaseEvent.await(30, TimeUnit.SECONDS))
    }
    val executor = Executors.newSingleThreadExecutor()
    val processor = QbftEventProcessor(queue, QbftEventMultiplexer(handler), executor)
    try {
      repeat(2) { run ->
        eventStarted = CountDownLatch(1)
        releaseEvent = CountDownLatch(1)
        val previousCompletion = processor.stop()
        processor.start()
        queue.add(BlockTimerExpiry(ConsensusRoundIdentifier(1, run)))
        try {
          assertThat(eventStarted.await(30, TimeUnit.SECONDS)).isTrue()
          val completion = processor.stop()
          assertThat(completion).isNotSameAs(previousCompletion).isNotDone()
          assertThat(processor.stop()).isSameAs(completion)
          assertThatThrownBy { processor.start() }.isInstanceOf(IllegalStateException::class.java)
        } finally {
          releaseEvent.countDown()
        }
        executor.submit {}.get(30, TimeUnit.SECONDS)
        assertThat(processor.stop()).isCompletedWithValue(Unit)
        queue.add(RoundExpiry(ConsensusRoundIdentifier(1, run)))
        assertThat(queue.isEmpty).isTrue()
      }
    } finally {
      releaseEvent.countDown()
      processor.stop()
      executor.shutdownNow()
    }
  }

  @Test
  fun `processing and cleanup failures complete stop exceptionally`() {
    val processingFailure = IllegalStateException("queue failed to start")
    val cleanupFailure = IllegalStateException("queue failed to stop")
    val queue = object : BftEventQueue(1000) {
      override fun start() {
        throw processingFailure
      }

      override fun stop() {
        throw cleanupFailure
      }
    }
    val executor = Executors.newSingleThreadExecutor()
    val processor = QbftEventProcessor(queue, QbftEventMultiplexer(FakeQbftEventHandler()), executor)
    try {
      processor.start()

      val completion = processor.stop()
      assertThatThrownBy { completion.get(30, TimeUnit.SECONDS) }.hasCause(processingFailure)
      assertThat(completion).isCompletedExceptionally()
      assertThat(processingFailure.suppressed).containsExactly(cleanupFailure)
    } finally {
      executor.shutdownNow()
    }
  }

  @Test
  fun `executor rejection completes stop exceptionally`() {
    val executor = Executors.newSingleThreadExecutor().also { it.shutdown() }
    val processor =
      QbftEventProcessor(
        BftEventQueue(1000),
        QbftEventMultiplexer(FakeQbftEventHandler()),
        executor,
      )

    assertThatThrownBy { processor.start() }.isInstanceOf(RejectedExecutionException::class.java)

    assertThat(processor.stop()).isCompletedExceptionally()
    assertThatThrownBy { processor.stop().get() }.hasCauseInstanceOf(RejectedExecutionException::class.java)
  }
}
