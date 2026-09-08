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
import org.hyperledger.besu.consensus.common.bft.BftExecutors
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.events.BlockTimerExpiry
import org.hyperledger.besu.metrics.noop.NoOpMetricsSystem
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import java.util.concurrent.CountDownLatch
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import kotlin.time.Duration.Companion.milliseconds

class QbftConsensusValidatorTest {
  private val eventQueueExecutor = Executors.newSingleThreadExecutor()
  private val bftExecutors = BftExecutors.create(NoOpMetricsSystem(), BftExecutors.ConsensusType.QBFT)

  @AfterEach
  fun tearDown() {
    eventQueueExecutor.shutdownNow()
    bftExecutors.stop()
  }

  @Test
  fun `repeated pause after timeout waits for the in-flight event before stopping resources`() {
    val importStarted = CountDownLatch(1)
    val releaseImport = CountDownLatch(1)
    val importFinished = CountDownLatch(1)
    val controller = FakeQbftEventHandler {
      importStarted.countDown()
      check(releaseImport.await(30, TimeUnit.SECONDS))
      importFinished.countDown()
    }
    val queue = BftEventQueue(1000)
    val processor = QbftEventProcessor(queue, QbftEventMultiplexer(controller))
    val validator = QbftConsensusValidator(controller, processor, bftExecutors, eventQueueExecutor, 10.milliseconds)
    try {
      validator.start()
      queue.add(BlockTimerExpiry(ConsensusRoundIdentifier(1, 0)))
      assertThat(importStarted.await(30, TimeUnit.SECONDS)).isTrue()

      repeat(2) {
        assertThatThrownBy { validator.pause() }.isInstanceOf(TimeoutException::class.java)
      }
      assertThat(processor.stop()).isNotDone()
      assertThat(controller.stops).isZero()
      bftExecutors.scheduleTask({}, 0, TimeUnit.MILLISECONDS).get(30, TimeUnit.SECONDS)
      validator.start()
      assertThat(controller.starts).isEqualTo(1)

      releaseImport.countDown()
      processor.stop().get(30, TimeUnit.SECONDS)
      validator.pause()

      assertThat(importFinished.count).isZero()
      assertThat(controller.stops).isEqualTo(1)
      assertThatThrownBy { bftExecutors.scheduleTask({}, 0, TimeUnit.MILLISECONDS) }
        .isInstanceOf(IllegalStateException::class.java)
    } finally {
      releaseImport.countDown()
      processor.stop().get(30, TimeUnit.SECONDS)
    }
  }

  @Test
  fun `pause completes when the validator was never started`() {
    val controller = FakeQbftEventHandler()
    val processor = QbftEventProcessor(BftEventQueue(1000), QbftEventMultiplexer(controller))
    val validator = QbftConsensusValidator(controller, processor, bftExecutors, eventQueueExecutor, 10.milliseconds)

    validator.pause()

    assertThat(processor.stop()).isCompletedWithValue(Unit)
  }

  @Test
  fun `processor failure is propagated after stopping validator resources`() {
    val failure = IllegalStateException("queue failed to start")
    val queue = object : BftEventQueue(1000) {
      override fun start() {
        throw failure
      }
    }
    val controller = FakeQbftEventHandler()
    val processor = QbftEventProcessor(queue, QbftEventMultiplexer(controller))
    val validator = QbftConsensusValidator(controller, processor, bftExecutors, { it.run() })
    validator.start()

    assertThatThrownBy { validator.pause() }
      .isInstanceOf(ExecutionException::class.java)
      .hasCause(failure)
    assertThat(controller.stops).isEqualTo(1)
    assertThatThrownBy { bftExecutors.scheduleTask({}, 0, TimeUnit.MILLISECONDS) }
      .isInstanceOf(IllegalStateException::class.java)
  }
}
