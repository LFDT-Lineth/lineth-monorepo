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
import org.hyperledger.besu.consensus.qbft.core.statemachine.QbftController
import org.hyperledger.besu.consensus.qbft.core.types.QbftEventHandler
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.time.Duration.Companion.seconds

class QbftConsensusValidatorTest {
  private val handlerStarted = CountDownLatch(1)
  private val handlerFinished = AtomicBoolean(false)
  private val eventQueue = BftEventQueue(1000)
  private val eventQueueExecutor = Executors.newSingleThreadExecutor()

  private fun createValidator(eventHandler: QbftEventHandler): QbftConsensusValidator =
    QbftConsensusValidator(
      qbftController = mock<QbftController>(),
      eventProcessor = QbftEventProcessor(eventQueue, QbftEventMultiplexer(eventHandler)),
      bftExecutors = mock<BftExecutors>(),
      eventQueueExecutor = eventQueueExecutor,
      shutdownTimeout = 30.seconds,
    )

  /**
   * ProtocolStarter closes the outgoing protocol and immediately starts the incoming one. Block
   * import runs synchronously on the BFT event thread, so if pause() returned while an import was
   * still in flight the incoming protocol would read a stale chain head and propose a duplicate
   * block at the same height.
   */
  @Test
  fun `pause blocks until the in-flight event has been fully handled`() {
    val eventHandler =
      mock<QbftEventHandler>().also {
        whenever(it.handleBlockTimerExpiry(any())).thenAnswer {
          handlerStarted.countDown()
          Thread.sleep(500)
          handlerFinished.set(true)
          null
        }
      }
    val validator = createValidator(eventHandler)
    try {
      validator.start()
      eventQueue.start()
      eventQueue.add(BlockTimerExpiry(ConsensusRoundIdentifier(1, 0)))
      assertThat(handlerStarted.await(30, TimeUnit.SECONDS)).isTrue()

      validator.pause()

      assertThat(handlerFinished).isTrue()
    } finally {
      eventQueueExecutor.shutdownNow()
    }
  }

  @Test
  fun `pause returns promptly when the validator was never started`() {
    val validator = createValidator(mock<QbftEventHandler>())
    try {
      val elapsed = System.nanoTime().let {
        validator.pause()
        System.nanoTime() - it
      }

      assertThat(elapsed).isLessThan(5.seconds.inWholeNanoseconds)
    } finally {
      eventQueueExecutor.shutdownNow()
    }
  }
}
