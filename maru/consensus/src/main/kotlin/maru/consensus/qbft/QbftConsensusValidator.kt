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
import org.hyperledger.besu.consensus.common.bft.BftExecutors
import org.hyperledger.besu.consensus.qbft.core.statemachine.QbftController
import java.util.concurrent.Executor

class QbftConsensusValidator(
  private val qbftController: QbftController,
  private val eventProcessor: QbftEventProcessor,
  private val bftExecutors: BftExecutors,
  private val eventQueueExecutor: Executor,
) : Protocol {
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
    eventProcessor.stop()
    bftExecutors.stop()
    qbftController.stop()
    isRunning = false
  }

  override fun close() {
    pause()
  }
}
