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
import org.junit.jupiter.api.Test
import org.mockito.kotlin.doAnswer
import org.mockito.kotlin.doThrow
import org.mockito.kotlin.mock
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit

class QbftEventProcessorTest {
  private val queue = mock<BftEventQueue>()
  private val multiplexer = mock<QbftEventMultiplexer>()
  private val processor = QbftEventProcessor(queue, multiplexer)

  @Test
  fun `stop is already complete before the first run`() {
    assertThat(processor.stop()).isDone()
  }

  @Test
  fun `each run has its own completion and finishes the current event before stopping`() {
    val executor = Executors.newSingleThreadExecutor()
    try {
      repeat(2) { run ->
        val event = BlockTimerExpiry(ConsensusRoundIdentifier(1, run))
        val eventStarted = CountDownLatch(1)
        val releaseEvent = CountDownLatch(1)
        whenever(queue.poll(500, TimeUnit.MILLISECONDS)).thenReturn(event)
        doAnswer {
          eventStarted.countDown()
          check(releaseEvent.await(30, TimeUnit.SECONDS))
          null
        }.whenever(multiplexer).handleEvent(event)

        val task = processor.start()
        val execution = executor.submit(task)
        try {
          assertThat(eventStarted.await(30, TimeUnit.SECONDS)).isTrue()
          val completion = processor.stop()
          assertThat(completion).isNotDone()
          assertThat(processor.stop()).isSameAs(completion)
          assertThatThrownBy { processor.start() }.isInstanceOf(IllegalStateException::class.java)
          verify(queue, times(run)).stop()
        } finally {
          releaseEvent.countDown()
        }
        execution.get(30, TimeUnit.SECONDS)
        assertThat(processor.stop()).isCompletedWithValue(Unit)
        verify(queue, times(run + 1)).stop()
      }
    } finally {
      executor.shutdownNow()
    }
  }

  @Test
  fun `processing and cleanup failures complete stop exceptionally`() {
    val processingFailure = IllegalStateException("queue failed to start")
    val cleanupFailure = IllegalStateException("queue failed to stop")
    doThrow(processingFailure).whenever(queue).start()
    doThrow(cleanupFailure).whenever(queue).stop()

    processor.start().run()

    assertThat(processor.stop()).isCompletedExceptionally()
    assertThatThrownBy { processor.stop().get() }.hasCause(processingFailure)
    assertThat(processingFailure.suppressed).containsExactly(cleanupFailure)
    verify(queue).stop()
  }
}
