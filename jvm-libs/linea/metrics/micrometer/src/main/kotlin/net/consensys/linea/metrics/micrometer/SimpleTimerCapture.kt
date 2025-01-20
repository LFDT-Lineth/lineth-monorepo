package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.Clock
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.Timer
import net.consensys.linea.metrics.TimerCapture
import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture

/**
 * An abstract class which contains everything is needed to make pretty much any Timer capture.
 * Mimicks the Builder pattern and allows captureTime implementation to perform different kinds of
 * captures TODO: In order to improve performance, Timer instances can be cached into a thread safe
 * Map
 */
class SimpleTimerCapture<T> : AbstractTimerCapture<T>, TimerCapture<T> {
  constructor(meterRegistry: MeterRegistry, name: String) : super(meterRegistry, name)
  constructor(
    meterRegistry: MeterRegistry,
    timerBuilder: Timer.Builder
  ) : super(meterRegistry, timerBuilder)

  override fun setDescription(description: String): SimpleTimerCapture<T> {
    super.setDescription(description)
    return this
  }

  override fun setTag(tagKey: String, tagValue: String): SimpleTimerCapture<T> {
    super.setTag(tagKey, tagValue)
    return this
  }

  override fun setClock(clock: Clock): SimpleTimerCapture<T> {
    super.setClock(clock)
    return this
  }

  override fun captureTime(f: CompletableFuture<T>): CompletableFuture<T> {
    val timer = timerBuilder.register(meterRegistry)
    val timerSample = Timer.start(clock)
    f.whenComplete { _, _ -> timerSample.stop(timer) }
    return f
  }

  override fun captureTime(action: Callable<T>): T {
    val timer = timerBuilder.register(meterRegistry)
    val timerSample = Timer.start(clock)
    val result = action.call()
    timerSample.stop(timer)
    return result
  }
}
