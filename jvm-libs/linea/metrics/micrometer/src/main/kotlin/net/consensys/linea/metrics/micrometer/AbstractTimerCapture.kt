package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.Clock
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.Timer
import io.vertx.core.Future
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture

/**
 * An abstract class which contains everything is needed to make pretty much any Timer capture.
 * Mimicks the Builder pattern and allows captureTime implementation to perform different kinds of
 * captures TODO: In order to improve performance, TimerCapture instances can be cached into a
 * thread safe Map just like it's made in MeterRegistry class
 */
abstract class AbstractTimerCapture<T> {
  protected val meterRegistry: MeterRegistry
  protected var timerBuilder: Timer.Builder
  protected var clock = Clock.SYSTEM

  constructor(meterRegistry: MeterRegistry, name: String) {
    this.meterRegistry = meterRegistry
    timerBuilder = Timer.builder(name)
  }

  constructor(meterRegistry: MeterRegistry, timerBuilder: Timer.Builder) {
    this.meterRegistry = meterRegistry
    this.timerBuilder = timerBuilder
  }

  open fun setDescription(description: String): AbstractTimerCapture<T> {
    timerBuilder.description(description)
    return this
  }

  open fun setTag(tagKey: String, tagValue: String): AbstractTimerCapture<T> {
    timerBuilder.tag(tagKey, tagValue)
    return this
  }

  open fun setClock(clock: Clock): AbstractTimerCapture<T> {
    this.clock = clock
    return this
  }

  abstract fun captureTime(action: Callable<T>): T
  abstract fun captureTime(f: CompletableFuture<T>): CompletableFuture<T>
  open fun captureTime(f: SafeFuture<T>): SafeFuture<T> {
    captureTime(f.toCompletableFuture())
    return f
  }
  open fun captureTime(f: Future<T>): Future<T> {
    captureTime(f.toCompletionStage().toCompletableFuture())
    return f
  }
}
