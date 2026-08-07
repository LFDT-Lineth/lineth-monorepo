package lineth.metrics.micrometer

import io.micrometer.core.instrument.MeterRegistry
import lineth.metrics.Counter
import lineth.metrics.CounterFactory
import lineth.metrics.Tag
import io.micrometer.core.instrument.Counter as MicrometerCounter

class MicrometerCounterAdapter(private val adaptee: MicrometerCounter) : Counter {
  override fun increment(amount: Double) {
    adaptee.increment(amount)
  }

  override fun increment() {
    adaptee.increment()
  }
}

class CounterFactoryImpl(
  meterRegistry: MeterRegistry,
  name: String,
  description: String,
  commonTags: List<Tag>,
) : CounterFactory {

  init {
    commonTags.forEach { it.requireValidMicrometerName() }
  }

  private val counterProvider = MicrometerCounter.builder(name)
    .description(description)
    .tags(commonTags.toMicrometerTags())
    .withRegistry(meterRegistry)

  override fun create(tags: List<Tag>): Counter {
    tags.forEach { it.requireValidMicrometerName() }
    return MicrometerCounterAdapter(counterProvider.withTags(tags.toMicrometerTags()))
  }
}
