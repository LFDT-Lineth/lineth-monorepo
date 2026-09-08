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
import org.hyperledger.besu.consensus.qbft.core.types.QbftEventHandler
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.Executor
import java.util.concurrent.TimeUnit
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

  private var isRunning = false
  private var pendingStop: SafeFuture<Unit>? = null

  @Synchronized
  override fun start() {
    if (isRunning) {
      if (pendingStop?.isDone != true) {
        return
      }
      pause()
    }
    val eventProcessorTask = eventProcessor.start()
    bftExecutors.start()
    qbftController.start()
    eventQueueExecutor.execute(eventProcessorTask)
    isRunning = true
  }

  @Synchronized
  override fun pause() {
    val completion = eventProcessor.stop()
    pendingStop = completion
    try {
      completion.get(shutdownTimeout.inWholeMilliseconds, TimeUnit.MILLISECONDS)
    } catch (e: InterruptedException) {
      Thread.currentThread().interrupt()
      throw e
    } finally {
      if (completion.isDone) {
        bftExecutors.stop()
        qbftController.stop()
        isRunning = false
        pendingStop = null
      }
    }
  }

  override fun close() {
    pause()
  }
}
