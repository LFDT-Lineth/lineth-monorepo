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
import org.hyperledger.besu.consensus.common.bft.events.RoundExpiry
import org.hyperledger.besu.consensus.qbft.core.types.QbftEventHandler
import org.hyperledger.besu.consensus.qbft.core.types.QbftNewChainHead
import org.hyperledger.besu.consensus.qbft.core.types.QbftReceivedMessageEvent
import org.hyperledger.besu.metrics.noop.NoOpMetricsSystem
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.doAnswer
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.spy
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CountDownLatch
import java.util.concurrent.ExecutionException
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class QbftConsensusValidatorTest {
  private class BlockingImportEventHandler : QbftEventHandler {
    val importStarted = CountDownLatch(1)
    val releaseImport = CountDownLatch(1)

    @Volatile var importFinished = false

    override fun start() = Unit

    override fun stop() = Unit

    override fun handleMessageEvent(event: QbftReceivedMessageEvent) = Unit

    override fun handleNewBlockEvent(event: QbftNewChainHead) = Unit

    override fun handleRoundExpiry(event: RoundExpiry) = Unit

    override fun handleBlockTimerExpiry(event: BlockTimerExpiry) {
      importStarted.countDown()
      check(releaseImport.await(30, TimeUnit.SECONDS))
      importFinished = true
    }
  }

  private val eventQueue = BftEventQueue(1000)
  private val eventQueueExecutor = Executors.newSingleThreadExecutor()
  private val bftExecutors = BftExecutors.create(NoOpMetricsSystem(), BftExecutors.ConsensusType.QBFT)

  private fun createValidator(
    eventHandler: QbftEventHandler,
    processor: QbftEventProcessor = QbftEventProcessor(eventQueue, QbftEventMultiplexer(eventHandler)),
  ): QbftConsensusValidator =
    QbftConsensusValidator(
      qbftController = eventHandler,
      eventProcessor = processor,
      bftExecutors = bftExecutors,
      eventQueueExecutor = eventQueueExecutor,
      shutdownTimeout = 30.seconds,
    )

  @AfterEach
  fun tearDown() {
    eventQueueExecutor.shutdownNow()
  }

  @Test
  fun `pause blocks until the in-flight event has been fully handled`() {
    val eventHandler = BlockingImportEventHandler()
    val processor = spy(QbftEventProcessor(eventQueue, QbftEventMultiplexer(eventHandler)))
    val stopRequested = CountDownLatch(1)
    doAnswer {
      val completion = it.callRealMethod()
      stopRequested.countDown()
      completion
    }.whenever(processor).stop()
    val validator = createValidator(eventHandler, processor)
    val pauseExecutor = Executors.newSingleThreadExecutor()
    try {
      validator.start()
      eventQueue.start()
      eventQueue.add(BlockTimerExpiry(ConsensusRoundIdentifier(1, 0)))
      assertThat(eventHandler.importStarted.await(30, TimeUnit.SECONDS)).isTrue()

      val paused = pauseExecutor.submit { validator.pause() }
      assertThat(stopRequested.await(30, TimeUnit.SECONDS)).isTrue()
      assertThatThrownBy { paused.get(100, TimeUnit.MILLISECONDS) }.isInstanceOf(TimeoutException::class.java)
      eventHandler.releaseImport.countDown()
      paused.get(30, TimeUnit.SECONDS)

      assertThat(eventHandler.importFinished).isTrue()
    } finally {
      eventHandler.releaseImport.countDown()
      pauseExecutor.shutdownNow()
      validator.close()
    }
  }

  @Test
  fun `pause returns promptly when the validator was never started`() {
    val validator = createValidator(BlockingImportEventHandler())

    val elapsed = System.nanoTime().let {
      validator.pause()
      System.nanoTime() - it
    }

    assertThat(elapsed).isLessThan(5.seconds.inWholeNanoseconds)
  }

  @Test
  fun `repeated pause after timeout still waits for the processor before stopping resources`() {
    val completion = SafeFuture<Unit>()
    val processor = mock<QbftEventProcessor>()
    whenever(processor.start()).thenReturn(Runnable {})
    whenever(processor.stop()).thenReturn(completion)
    val controller = mock<QbftEventHandler>()
    val executors = mock<BftExecutors>()
    val validator = QbftConsensusValidator(controller, processor, executors, {}, 1.milliseconds)
    validator.start()

    repeat(2) {
      assertThatThrownBy { validator.pause() }.isInstanceOf(TimeoutException::class.java)
    }
    verify(controller, never()).stop()
    verify(executors, never()).stop()
    validator.start()
    verify(processor).start()

    completion.complete(Unit)
    validator.pause()
    verify(controller).stop()
    verify(executors).stop()
  }

  @Test
  fun `processor failure is propagated after stopping validator resources`() {
    val failure = IllegalStateException("processor failed")
    val processor = mock<QbftEventProcessor>()
    whenever(processor.stop()).thenReturn(SafeFuture.failedFuture(failure))
    val controller = mock<QbftEventHandler>()
    val executors = mock<BftExecutors>()
    val validator = QbftConsensusValidator(controller, processor, executors, {})

    assertThatThrownBy { validator.pause() }
      .isInstanceOf(ExecutionException::class.java)
      .hasCause(failure)
    verify(controller).stop()
    verify(executors).stop()
  }
}
