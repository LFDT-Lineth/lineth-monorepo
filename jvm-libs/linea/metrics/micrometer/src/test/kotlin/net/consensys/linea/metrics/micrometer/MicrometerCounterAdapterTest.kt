package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.Meter
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import net.consensys.linea.metrics.Tag
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.Test
import io.micrometer.core.instrument.Counter as MicrometerCounter

class MicrometerCounterAdapterTest {

  @Test
  fun `create counter with no extra tags`() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val counterFactory = CounterFactoryImpl(
      meterRegistry = meterRegistry,
      name = "request.counter",
      description = "API request counter",
      commonTags = listOf(Tag("version", "v1.0")),
    )
    val counter = counterFactory.create(tags = emptyList())
    counter.increment()
    counter.increment(2.0)

    Assertions.assertThat(meterRegistry.meters.size).isEqualTo(1)
    Assertions.assertThat(meterRegistry.meters.first().id.type).isEqualTo(Meter.Type.COUNTER)

    val micrometerCounter = meterRegistry.meters.first() as MicrometerCounter
    Assertions.assertThat(micrometerCounter.count()).isEqualTo(3.0)
    Assertions.assertThat(micrometerCounter.id.name).isEqualTo("request.counter")
    Assertions.assertThat(micrometerCounter.id.description).isEqualTo("API request counter")
    Assertions.assertThat(micrometerCounter.id.tags.size).isEqualTo(1)
    Assertions.assertThat(
      micrometerCounter.id.tags.flatMap { tag -> listOf(tag.key, tag.value) },
    ).containsExactly("version", "v1.0")
  }

  @Test
  fun `create multiple counter with extra tags`() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val counterFactory = CounterFactoryImpl(
      meterRegistry = meterRegistry,
      name = "request.counter",
      description = "API request counter",
      commonTags = listOf(Tag("version", "v1.0")),
    )

    counterFactory.create(listOf(Tag("counterid", "1"))).increment()
    counterFactory.create(listOf(Tag("counterid", "2"))).increment(2.0)
    counterFactory.create(listOf(Tag("counterid", "1"))).increment(3.0)

    Assertions.assertThat(meterRegistry.meters.size).isEqualTo(2)
    meterRegistry.meters.forEach { meter ->
      Assertions.assertThat(meter.id.type).isEqualTo(Meter.Type.COUNTER)
      Assertions.assertThat(meter.id.name).isEqualTo("request.counter")
      Assertions.assertThat(meter.id.description).isEqualTo("API request counter")
      Assertions.assertThat(meter.id.tags.map { it.key }).containsExactlyInAnyOrder("version", "counterid")
    }

    val counter1 = meterRegistry.find("request.counter").tags("counterid", "1").counter()
    Assertions.assertThat(counter1).isNotNull
    Assertions.assertThat(counter1!!.count()).isEqualTo(4.0)

    val counter2 = meterRegistry.find("request.counter").tags("counterid", "2").counter()
    Assertions.assertThat(counter2).isNotNull
    Assertions.assertThat(counter2!!.count()).isEqualTo(2.0)
  }
}
