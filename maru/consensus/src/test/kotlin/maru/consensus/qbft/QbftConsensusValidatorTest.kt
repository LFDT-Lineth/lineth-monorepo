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
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class QbftConsensusValidatorTest {
  /**
   * Stands in for QbftController, whose block import runs synchronously on the event processor
   * thread. [handleBlockTimerExpiry] mimics an import that is slow enough to still be in flight
   * when the protocol is closed.
   */
  private class SlowImportEventHandler(
    private val importDuration: Duration,
  ) : QbftEventHandler {
    val importStarted = CountDownLatch(1)

    @Volatile var importFinished = false

    override fun start() = Unit

    override fun stop() = Unit

    override fun handleMessageEvent(event: QbftReceivedMessageEvent) = Unit

    override fun handleNewBlockEvent(event: QbftNewChainHead) = Unit

    override fun handleRoundExpiry(event: RoundExpiry) = Unit

    override fun handleBlockTimerExpiry(event: BlockTimerExpiry) {
      importStarted.countDown()
      Thread.sleep(importDuration.inWholeMilliseconds)
      importFinished = true
    }
  }

  private val eventQueue = BftEventQueue(1000)
  private val eventQueueExecutor = Executors.newSingleThreadExecutor()
  private val bftExecutors = BftExecutors.create(NoOpMetricsSystem(), BftExecutors.ConsensusType.QBFT)

  private fun createValidator(eventHandler: QbftEventHandler): QbftConsensusValidator =
    QbftConsensusValidator(
      qbftController = eventHandler,
      eventProcessor = QbftEventProcessor(eventQueue, QbftEventMultiplexer(eventHandler)),
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
    val eventHandler = SlowImportEventHandler(importDuration = 500.milliseconds)
    val validator = createValidator(eventHandler)
    validator.start()
    eventQueue.start()
    eventQueue.add(BlockTimerExpiry(ConsensusRoundIdentifier(1, 0)))
    assertThat(eventHandler.importStarted.await(30, TimeUnit.SECONDS)).isTrue()

    validator.pause()

    assertThat(eventHandler.importFinished).isTrue()
  }

  @Test
  fun `pause returns promptly when the validator was never started`() {
    val validator = createValidator(SlowImportEventHandler(importDuration = 500.milliseconds))

    val elapsed = System.nanoTime().let {
      validator.pause()
      System.nanoTime() - it
    }

    assertThat(elapsed).isLessThan(5.seconds.inWholeNanoseconds)
  }
}
