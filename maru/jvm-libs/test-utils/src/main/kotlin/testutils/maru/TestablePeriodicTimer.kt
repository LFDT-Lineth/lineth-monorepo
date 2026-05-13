/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils.maru

import linea.timer.Timer
import linea.timer.TimerFactory
import linea.timer.TimerSchedule
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration

// Test implementation that allows controlling the timer execution. Supports only scheduleAtFixedRate
class TestablePeriodicTimer(
  override val name: String,
  override val initialDelay: Duration,
  override val period: Duration,
  override val task: Runnable,
) : Timer {
  override val errorHandler: (Throwable) -> Unit = { }
  override val timerSchedule: TimerSchedule = TimerSchedule.FIXED_RATE

  private val startCounter = AtomicInteger(0)
  public val startCount: Int
    get() = startCounter.get()

  private val stopCounter = AtomicInteger(0)
  public val stopCount: Int
    get() = stopCounter.get()

  override fun start() {
    startCounter.incrementAndGet()
  }

  fun runNextTask() {
    task.run()
  }

  override fun stop() {
    stopCounter.incrementAndGet()
  }
}

class TestablePeriodicTimerFactory : TimerFactory {
  val createdTimers = mutableMapOf<String, TestablePeriodicTimer>()

  override fun createTimer(
    name: String,
    initialDelay: Duration,
    period: Duration,
    timerSchedule: TimerSchedule,
    errorHandler: (Throwable) -> Unit,
    task: Runnable,
  ): Timer {
    val timer = TestablePeriodicTimer(name, initialDelay, period, task)
    createdTimers[name] = timer
    return timer
  }

  fun getTimer(name: String): TestablePeriodicTimer? = createdTimers[name]
}
