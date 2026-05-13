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
import org.hyperledger.besu.consensus.common.bft.BftEventQueue
import org.hyperledger.besu.consensus.common.bft.events.BftEvent
import java.util.Optional
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

class QbftEventProcessor(
  private val incomingQueue: BftEventQueue,
  private val eventMultiplexer: QbftEventMultiplexer,
) : Runnable {
  private val log: org.apache.logging.log4j.Logger = LogManager.getLogger(this.javaClass)
  private val shutdownLatch = CountDownLatch(1)

  @Volatile private var shutdown = false

  /**
   * Indicate to the processor that it can be started
   */
  @Synchronized
  fun start() {
    shutdown = false
  }

  /**
   * Indicate to the processor that it should gracefully stop at its next opportunity
   */
  @Synchronized
  fun stop() {
    shutdown = true
  }

  /**
   * Await stop.
   *
   * @throws InterruptedException the interrupted exception
   */
  @Throws(InterruptedException::class)
  fun awaitStop() {
    shutdownLatch.await()
  }

  override fun run() {
    try {
      // Start the event queue. Until it is started it won't accept new events from peers
      incomingQueue.start()

      while (!shutdown) {
        nextEvent().ifPresent { eventMultiplexer.handleEvent(it) }
      }

      incomingQueue.stop()
    } catch (t: Throwable) {
      log.error("BFT Mining thread has suffered a fatal error, mining has been halted", t)
    }

    // Clean up the executor service the round timer has been utilising
    log.info("Shutting down BFT event processor")
    shutdownLatch.countDown()
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
