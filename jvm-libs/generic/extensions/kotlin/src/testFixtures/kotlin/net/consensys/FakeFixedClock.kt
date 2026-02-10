package net.consensys

import kotlin.time.Clock
import kotlin.time.Instant

/**
 * Kotlin Clock Mocking class to make unittest easier and less boilerplate
 */

class FakeFixedClock(
  private var time: Instant = Clock.System.now(),
) : Clock {

  @Synchronized
  override fun now(): Instant = time

  @Synchronized
  fun advanceBy(duration: kotlin.time.Duration) {
    time = time.plus(duration)
  }

  @Synchronized
  fun goBackBy(duration: kotlin.time.Duration) {
    time = time.minus(duration)
  }

  @Synchronized
  fun setTimeTo(instant: Instant) {
    time = instant
  }
}
