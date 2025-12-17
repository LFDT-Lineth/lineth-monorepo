package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.Clock
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.Timer
import net.consensys.linea.metrics.DynamicTagTimer
import net.consensys.linea.metrics.Tag
import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture
import io.micrometer.core.instrument.Timer as MicrometerTimer

/**
 * This class can tag a metric with a tag which is unknown at the start of a measured call, but can
 * be derived from its result. Please note that the extraction logic is included in the measurement
 * due to limitations of Micrometer design, thus, this class is not suitable for the precise
 * nanosecond-level measurements. Related issue:
 * https://github.com/micrometer-metrics/micrometer/issues/535
 */
class DynamicTagTimerImpl<T>(
  private val meterRegistry: MeterRegistry,
  name: String,
  description: String,
  commonTags: List<Tag>,
  private val clock: Clock = Clock.SYSTEM,
  private val extractor: (T) -> List<Tag>,
  private val extractorOnError: (Throwable) -> List<Tag>,
) : DynamicTagTimer<T> {

  private val timerBuilder: MicrometerTimer.Builder = MicrometerTimer.builder(name)
    .description(description)
    .tags(*commonTags.flatMap { listOf(it.key, it.value) }.toTypedArray())

  private fun getTimer(dynamicTags: List<Tag>): MicrometerTimer {
    dynamicTags.forEach { it.requireValidMicrometerName() }
    return timerBuilder
      .tags(*dynamicTags.flatMap { listOf(it.key, it.value) }.toTypedArray())
      .register(meterRegistry)
  }

  override fun captureTime(f: CompletableFuture<T>): CompletableFuture<T> {
    val timerSample = Timer.start(clock)
    f.whenComplete { result: T?, error: Throwable? ->
      val dynamicTags = result?.let { extractor.invoke(it) } ?: extractorOnError.invoke(error!!)
      dynamicTags.forEach { it.requireValidMicrometerName() }
      val timer = getTimer(dynamicTags)
      timerSample.stop(timer)
    }
    return f
  }

  override fun captureTime(action: Callable<T>): T {
    val timerSample = Timer.start(clock)
    val result = runCatching { action.call() }
    val dynamicTags = result.fold(
      onSuccess = { extractor.invoke(it) },
      onFailure = { extractorOnError.invoke(it) },
    )
    dynamicTags.forEach { it.requireValidMicrometerName() }
    val timer = getTimer(dynamicTags)
    timerSample.stop(timer)
    return result.getOrThrow()
  }
}
