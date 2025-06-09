package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.Clock
import io.micrometer.core.instrument.MeterRegistry
import net.consensys.linea.metrics.Tag
import net.consensys.linea.metrics.Timer
import net.consensys.linea.metrics.TimerFactory
import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture
import io.micrometer.core.instrument.Timer as MicrometerTimer

class TimerAdapter(val adaptee: MicrometerTimer, val clock: Clock) : Timer {
  override fun <T> captureTime(f: CompletableFuture<T>): CompletableFuture<T> {
    val timerSample = MicrometerTimer.start(clock)
    f.whenComplete { _, _ -> timerSample.stop(adaptee) }
    return f
  }

  override fun <T> captureTime(
    action: Callable<T>,
  ): T {
    val timerSample = MicrometerTimer.start(clock)
    try {
      return action.call()
    } finally {
      timerSample.stop(adaptee)
    }
  }
}

class TimerFactoryImpl(
  meterRegistry: MeterRegistry,
  name: String,
  description: String,
  commonTags: List<Tag>,
  private val clock: Clock = Clock.SYSTEM,
) : TimerFactory {

  private val timerProvider = MicrometerTimer.builder(name)
    .description(description)
    .tags(*commonTags.flatMap { listOf(it.key, it.value) }.toTypedArray())
    .withRegistry(meterRegistry)

  init {
    commonTags.forEach { it.requireValidMicrometerName() }
  }

  fun getTimer(tags: List<Tag>): MicrometerTimer {
    tags.forEach { it.requireValidMicrometerName() }
    return timerProvider.withTags(*tags.flatMap { listOf(it.key, it.value) }.toTypedArray())
  }

  override fun create(tags: List<Tag>): Timer {
    return TimerAdapter(getTimer(tags), clock)
  }
}
