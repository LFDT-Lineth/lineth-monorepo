/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import linea.timer.TestablePeriodicTimerFactory
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.ProtocolFactory
import maru.consensus.ProtocolStarter
import maru.consensus.QbftConsensusConfig
import maru.core.ext.DataGenerators
import maru.subscription.InOrderFanoutSubscriptionManager
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.consensus.common.bft.BftEventQueue
import org.hyperledger.besu.consensus.common.bft.BftExecutors
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.events.BlockTimerExpiry
import org.hyperledger.besu.metrics.noop.NoOpMetricsSystem
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.ValueSource
import java.util.concurrent.CountDownLatch
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class QbftConsensusValidatorTest {
  private val eventQueueExecutor = Executors.newSingleThreadExecutor()
  private val bftExecutors = BftExecutors.create(NoOpMetricsSystem(), BftExecutors.ConsensusType.QBFT)

  @AfterEach
  fun tearDown() {
    eventQueueExecutor.shutdownNow()
    bftExecutors.stop()
  }

  @Test
  fun `restart after timed-out pause waits for the in-flight event and cleans up the previous run`() {
    val importStarted = CountDownLatch(1)
    val releaseImport = CountDownLatch(1)
    val importFinished = CountDownLatch(1)
    val restartedEventHandled = CountDownLatch(1)
    val controller = FakeQbftEventHandler { event ->
      if (event.roundIdentifier.sequenceNumber == 2L) {
        restartedEventHandled.countDown()
      } else {
        importStarted.countDown()
        check(releaseImport.await(30, TimeUnit.SECONDS))
        importFinished.countDown()
      }
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
      validator.start()

      assertThat(importFinished.count).isZero()
      assertThat(controller.stops).isEqualTo(1)
      assertThat(controller.starts).isEqualTo(2)
      queue.add(BlockTimerExpiry(ConsensusRoundIdentifier(2, 0)))
      assertThat(restartedEventHandled.await(30, TimeUnit.SECONDS)).isTrue()
      bftExecutors.scheduleTask({}, 0, TimeUnit.MILLISECONDS).get(30, TimeUnit.SECONDS)

      processor.stop().get(30, TimeUnit.SECONDS)
      validator.pause()
      assertThat(controller.stops).isEqualTo(2)
      assertThatThrownBy { bftExecutors.scheduleTask({}, 0, TimeUnit.MILLISECONDS) }
        .isInstanceOf(IllegalStateException::class.java)
    } finally {
      releaseImport.countDown()
      processor.stop().get(30, TimeUnit.SECONDS)
    }
  }

  @ParameterizedTest
  @ValueSource(booleans = [false, true])
  fun `timed-out close cleans up after the event finishes without a lifecycle retry`(failEvent: Boolean) {
    val eventStarted = CountDownLatch(1)
    val releaseEvent = CountDownLatch(1)
    val controller = FakeQbftEventHandler {
      eventStarted.countDown()
      check(releaseEvent.await(30, TimeUnit.SECONDS))
      if (failEvent) {
        throw AssertionError("event failed")
      }
    }
    val queue = BftEventQueue(1000)
    val processor = QbftEventProcessor(queue, QbftEventMultiplexer(controller))
    val validator = QbftConsensusValidator(controller, processor, bftExecutors, eventQueueExecutor, 10.milliseconds)
    try {
      validator.start()
      queue.add(BlockTimerExpiry(ConsensusRoundIdentifier(1, 0)))
      assertThat(eventStarted.await(30, TimeUnit.SECONDS)).isTrue()

      repeat(2) {
        assertThatThrownBy { validator.close() }.isInstanceOf(TimeoutException::class.java)
      }
      assertThat(controller.stops).isZero()
      bftExecutors.scheduleTask({}, 0, TimeUnit.MILLISECONDS).get(30, TimeUnit.SECONDS)

      releaseEvent.countDown()
      eventQueueExecutor.submit {}.get(30, TimeUnit.SECONDS)

      assertThat(controller.stops).isEqualTo(1)
      assertThatThrownBy { bftExecutors.scheduleTask({}, 0, TimeUnit.MILLISECONDS) }
        .isInstanceOf(IllegalStateException::class.java)
      validator.close()
      assertThat(controller.stops).isEqualTo(1)
    } finally {
      releaseEvent.countDown()
      eventQueueExecutor.submit {}.get(30, TimeUnit.SECONDS)
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
  fun `fork transition retries successfully after failed event processor cleanup`() {
    val failure = IllegalStateException("queue failed to start")
    val queue = object : BftEventQueue(1000) {
      override fun start() {
        throw failure
      }
    }
    val controller = FakeQbftEventHandler()
    val processor = QbftEventProcessor(queue, QbftEventMultiplexer(controller))
    val failedValidator = QbftConsensusValidator(controller, processor, bftExecutors, { it.run() })
    val nextController = FakeQbftEventHandler()
    val nextProcessor = QbftEventProcessor(BftEventQueue(1000), QbftEventMultiplexer(nextController))
    val nextValidator = QbftConsensusValidator(nextController, nextProcessor, bftExecutors, eventQueueExecutor)
    val validators = DataGenerators.randomValidators()
    val oldFork = ForkSpec(0UL, 1u, QbftConsensusConfig(validators, ChainFork(ClFork.QBFT_PHASE0, ElFork.Osaka)))
    val nextFork = ForkSpec(10UL, 1u, QbftConsensusConfig(validators, ChainFork(ClFork.QBFT_PHASE0, ElFork.Amsterdam)))
    var nextTimestamp = 0UL
    val starter = ProtocolStarter(
      forksSchedule = ForksSchedule(1337u, listOf(oldFork, nextFork)),
      protocolFactory = object : ProtocolFactory {
        override fun create(forkSpec: ForkSpec) = if (forkSpec == oldFork) failedValidator else nextValidator
      },
      nextBlockTimestampProvider = { nextTimestamp },
      forkTransitionCheckInterval = 1.seconds,
      timerFactory = TestablePeriodicTimerFactory(),
      forkTransitionNotifier = InOrderFanoutSubscriptionManager(),
    )
    try {
      starter.start()
      nextTimestamp = nextFork.timestampSeconds
      assertThatThrownBy { starter.start() }.isInstanceOf(ExecutionException::class.java).hasCause(failure)

      starter.start()

      assertThat(starter.currentProtocolWithForkReference.get().fork).isEqualTo(nextFork)
      assertThat(nextController.starts).isEqualTo(1)
      assertThat(controller.stops).isEqualTo(1)
    } finally {
      starter.close()
    }
  }

  @Test
  fun `processor failure is propagated once and subsequent shutdowns succeed`() {
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
    repeat(2) {
      validator.close()
      validator.pause()
    }
    assertThat(controller.stops).isEqualTo(1)
    assertThatThrownBy { bftExecutors.scheduleTask({}, 0, TimeUnit.MILLISECONDS) }
      .isInstanceOf(IllegalStateException::class.java)
  }
}
