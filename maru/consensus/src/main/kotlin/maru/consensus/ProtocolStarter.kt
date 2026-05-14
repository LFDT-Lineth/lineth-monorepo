/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import linea.timer.Timer
import linea.timer.TimerFactory
import maru.core.Protocol
import maru.subscription.SubscriptionNotifier
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.time.Clock
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration

class ProtocolStarter(
  private val forksSchedule: ForksSchedule,
  private val protocolFactory: ProtocolFactory,
  private val nextBlockTimestampProvider: NextBlockTimestampProvider,
  private val forkTransitionCheckInterval: Duration,
  private val clock: Clock = Clock.systemUTC(),
  timerFactory: TimerFactory,
  private val forkTransitionNotifier: SubscriptionNotifier<ForkSpec>,
) : Protocol {
  data class ProtocolWithFork(
    val protocol: Protocol,
    val fork: ForkSpec,
  ) {
    override fun toString(): String = "protocol=${protocol.javaClass.simpleName}, fork=$fork"
  }

  private val log: Logger = LogManager.getLogger(this.javaClass)

  internal val currentProtocolWithForkReference: AtomicReference<ProtocolWithFork> = AtomicReference()

  private var poller: Timer =
    timerFactory.createTimer(
      name = "ProtocolStarterPoller",
      initialDelay = forkTransitionCheckInterval,
      period = forkTransitionCheckInterval,
      timerSchedule = linea.timer.TimerSchedule.FIXED_RATE,
      errorHandler = {},
      task = { pollTask() },
    )

  private fun pollTask() {
    try {
      checkAndHandleForkTransition()
    } catch (th: Throwable) {
      log.error("Error during fork transition check", th)
    }
  }

  private fun checkAndHandleForkTransition(): Boolean {
    val currentTimestamp = clock.instant().epochSecond.toULong()
    val nextBlockTimestamp = nextBlockTimestampProvider.nextTargetBlockUnixTimestamp(currentTimestamp)
    val nextForkSpec = forksSchedule.getForkByTimestamp(nextBlockTimestamp)

    val currentProtocolWithFork = currentProtocolWithForkReference.get()

    if (currentProtocolWithFork?.fork != nextForkSpec) {
      log.debug(
        "switching from forkSpec={} to newForkSpec={}, nextBlockTimeStamp={}",
        currentProtocolWithFork?.fork,
        nextForkSpec,
        nextBlockTimestamp,
      )

      performForkTransition(currentProtocolWithFork, nextForkSpec)
      return true
    } else {
      log.trace("currentTimestamp={}, but fork switch isn't required", currentTimestamp)
      return false
    }
  }

  @Synchronized
  private fun performForkTransition(
    currentProtocolWithFork: ProtocolWithFork?,
    nextForkSpec: ForkSpec,
  ) {
    currentProtocolWithFork?.protocol?.close()

    val newProtocol: Protocol = protocolFactory.create(nextForkSpec)
    val newProtocolWithFork =
      ProtocolWithFork(
        newProtocol,
        nextForkSpec,
      )
    log.debug("switching protocol: fromProtocol={} toProtocol={}", currentProtocolWithFork, newProtocolWithFork)
    currentProtocolWithForkReference.set(newProtocolWithFork)

    newProtocol.start()
    log.debug("started new protocol {}", newProtocol)
    forkTransitionNotifier.notifySubscribers(nextForkSpec)
  }

  override fun start() {
    synchronized(this) {
      val transitioned = checkAndHandleForkTransition()
      if (!transitioned) {
        // Restart case: fork didn't change but protocol was paused — re-start it
        currentProtocolWithForkReference.get()?.protocol?.start()
      }
      poller.start()
      log.debug("Starting fork transition polling with interval {}", forkTransitionCheckInterval)
    }
  }

  override fun pause() {
    synchronized(this) {
      poller.stop()
      currentProtocolWithForkReference.get()?.protocol?.pause()
      log.debug("Stopped fork transition polling")
    }
  }

  override fun close() {
    synchronized(this) {
      pause()
      currentProtocolWithForkReference.get()?.protocol?.close()
    }
  }
}
