/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.consensus.common.bft.BftEventQueue
import org.hyperledger.besu.consensus.common.bft.events.BftEvent
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Optional
import java.util.concurrent.ExecutorService
import java.util.concurrent.TimeUnit

class QbftEventProcessor(
  private val incomingQueue: BftEventQueue,
  private val eventMultiplexer: QbftEventMultiplexer,
  private val executor: ExecutorService,
) {
  private val log: Logger = LogManager.getLogger(this.javaClass)
  private var shutdownCompletion = SafeFuture.completedFuture(Unit)

  @Volatile private var shutdown = false

  /** Start a new run after the previous one has stopped. */
  fun start() {
    val completion =
      synchronized(this) {
        check(shutdownCompletion.isDone) { "The previous event processor run has not stopped" }
        SafeFuture<Unit>().also {
          shutdownCompletion = it
          shutdown = false
        }
      }
    try {
      executor.execute { run(completion) }
    } catch (t: Throwable) {
      completion.completeExceptionally(t)
      throw t
    }
  }

  /** Complete once the current event has finished and the queue has stopped. */
  @Synchronized
  fun stop(): SafeFuture<Unit> {
    shutdown = true
    return shutdownCompletion
  }

  private fun run(completion: SafeFuture<Unit>) {
    var failure: Throwable? = null
    try {
      incomingQueue.start()
      while (!shutdown) {
        nextEvent().ifPresent { eventMultiplexer.handleEvent(it) }
      }
    } catch (t: Throwable) {
      failure = t
      log.error("BFT Mining thread has suffered a fatal error, mining has been halted", t)
    } finally {
      try {
        incomingQueue.stop()
      } catch (t: Throwable) {
        val processingFailure = failure
        if (processingFailure == null) {
          failure = t
        } else {
          processingFailure.addSuppressed(t)
        }
      } finally {
        val shutdownFailure = failure
        if (shutdownFailure == null) {
          completion.complete(Unit)
        } else {
          completion.completeExceptionally(shutdownFailure)
        }
      }
    }
  }

  private fun nextEvent(): Optional<BftEvent> =
    try {
      Optional.ofNullable(incomingQueue.poll(500, TimeUnit.MILLISECONDS))
    } catch (_: InterruptedException) {
      // If the queue was interrupted propagate it and spin to check our shutdown status
      Thread.currentThread().interrupt()
      Optional.empty()
    }
}
