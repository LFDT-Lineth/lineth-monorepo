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
import org.hyperledger.besu.consensus.qbft.core.statemachine.QbftController
import java.util.concurrent.Executor
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

class QbftConsensusValidator(
  private val qbftController: QbftController,
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
    // Block until the event processor has finished the event it is currently handling. Block import
    // runs synchronously on that thread, so returning early would let ProtocolStarter start the next
    // fork's protocol while this one is still committing a block. The incoming protocol would then
    // read a stale chain head and propose a second block at the same height, forking the chain and
    // stranding followers that already imported the outgoing fork's block.
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
