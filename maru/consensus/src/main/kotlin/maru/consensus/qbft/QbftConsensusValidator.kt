/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.core.Protocol
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.consensus.common.bft.BftExecutors
import org.hyperledger.besu.consensus.qbft.core.types.QbftEventHandler
import java.util.concurrent.Executor
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

class QbftConsensusValidator(
  private val qbftController: QbftEventHandler,
  private val eventProcessor: QbftEventProcessor,
  private val bftExecutors: BftExecutors,
  private val eventQueueExecutor: Executor,
  private val shutdownTimeout: Duration = DEFAULT_SHUTDOWN_TIMEOUT,
) : Protocol {
  companion object {
    val DEFAULT_SHUTDOWN_TIMEOUT: Duration = 30.seconds
  }

  private val log = LogManager.getLogger(this.javaClass)
  private var isRunning = false

  @Synchronized
  override fun start() {
    if (isRunning) {
      return
    }
    eventProcessor.start()
    bftExecutors.start()
    qbftController.start()
    eventQueueExecutor.execute(eventProcessor)
    isRunning = true
  }

  @Synchronized
  override fun pause() {
    val wasRunning = isRunning
    isRunning = false
    eventProcessor.stop()
    // Returning before the in-flight block import commits lets ProtocolStarter start the next
    // fork's protocol against a stale chain head, forking the chain at that height.
    if (wasRunning && !eventProcessor.awaitStop(shutdownTimeout)) {
      log.warn("BFT event processor did not stop within {}, proceeding with shutdown anyway", shutdownTimeout)
    }
    bftExecutors.stop()
    qbftController.stop()
  }

  override fun close() {
    pause()
  }
}
