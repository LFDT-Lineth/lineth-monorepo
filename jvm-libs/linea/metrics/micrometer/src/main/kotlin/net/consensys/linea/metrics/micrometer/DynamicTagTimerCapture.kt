package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.Clock
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.Timer
import net.consensys.linea.metrics.TimerCapture
import java.util.concurrent.Callable
import java.util.concurrent.CompletableFuture
import java.util.function.Function

/**
 * This class can tag a metric with a tag which is unknown at the start of a measured call, but can
 * be derived from its result. Please note that the extraction logic is included in the measurement
 * due to limitations of Micrometer design, thus, this class is not suitable for the precise
 * nanosecond-level measurements. Related issue:
 * https://github.com/micrometer-metrics/micrometer/issues/535
 */
class DynamicTagTimerCapture<T> : AbstractTimerCapture<T>, TimerCapture<T> {
  private var extractor: Function<T, String>? = null
  private var extractorOnError: Function<Throwable, String>? = null
  private var tagKey: String? = null

  constructor(meterRegistry: MeterRegistry, name: String) : super(meterRegistry, name)
  constructor(
    meterRegistry: MeterRegistry,
    timerBuilder: Timer.Builder
  ) : super(meterRegistry, timerBuilder)

  override fun setDescription(description: String): DynamicTagTimerCapture<T> {
    super.setDescription(description)
    return this
  }

  override fun setTag(tagKey: String, tagValue: String): DynamicTagTimerCapture<T> {
    throw NoSuchMethodException(
      "If you need to set both value and key, please use ${SimpleTimerCapture::class.qualifiedName}"
    )
  }

  override fun setClock(clock: Clock): DynamicTagTimerCapture<T> {
    super.setClock(clock)
    return this
  }

  fun setTagKey(tagKey: String?): DynamicTagTimerCapture<T> {
    this.tagKey = tagKey
    return this
  }

  fun setTagValueExtractor(extractor: Function<T, String>): DynamicTagTimerCapture<T> {
    this.extractor = extractor
    return this
  }

  fun setTagValueExtractorOnError(
    onErrorExtractor: Function<Throwable, String>
  ): DynamicTagTimerCapture<T> {
    this.extractorOnError = onErrorExtractor
    return this
  }

  private fun ensureValidState() {
    assert(extractor != null)
    assert(extractorOnError != null)
    assert(tagKey != null)
  }

  override fun captureTime(f: CompletableFuture<T>): CompletableFuture<T> {
    ensureValidState()
    val timerSample = Timer.start(clock)
    f.whenComplete { result: T?, error: Throwable? ->
      val tagValue: String = result?.let(extractor!!::apply) ?: extractorOnError!!.apply(error!!)
      val timer = timerBuilder.tag(tagKey!!, tagValue).register(meterRegistry)
      timerSample.stop(timer)
    }
    return f
  }

  override fun captureTime(action: Callable<T>): T {
    ensureValidState()
    val timerSample = Timer.start(clock)
    val result = action.call()
    val labelValue = extractor!!.apply(result)
    val timer = timerBuilder.tag(tagKey!!, labelValue).register(meterRegistry)
    timerSample.stop(timer)
    return result
  }
}
