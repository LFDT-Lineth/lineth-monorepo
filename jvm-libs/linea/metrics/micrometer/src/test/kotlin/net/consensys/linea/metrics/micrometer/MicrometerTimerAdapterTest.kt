package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.Meter
import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.MockClock
import io.micrometer.core.instrument.Timer
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import net.consensys.linea.metrics.Tag
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeUnit

class MicrometerTimerAdapterTest {

  @Test
  fun `create timer with no extra tags`() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val apiTimerProvider =
      TimerFactoryImpl(
        meterRegistry = meterRegistry,
        name = "request.timer",
        description = "API request timer",
        commonTags = listOf(Tag("version", "v1.0")),
      )
    val result = apiTimerProvider.create(tags = emptyList())
      .captureTime {
        Thread.sleep(10)
        "result"
      }
    Assertions.assertThat(result).isEqualTo("result")
    Assertions.assertThat(meterRegistry.meters.size).isEqualTo(1)

    Assertions.assertThat(meterRegistry.meters.first().id.type).isEqualTo(Meter.Type.TIMER)
    val timer = meterRegistry.meters.first() as Timer
    Assertions.assertThat(timer.id.name).isEqualTo("request.timer")
    Assertions.assertThat(timer.id.description).isEqualTo("API request timer")
    Assertions.assertThat(timer.id.tags.size).isEqualTo(1)
    Assertions.assertThat(
      timer.id.tags.flatMap { tag -> listOf(tag.key, tag.value) },
    ).containsExactly("version", "v1.0")
  }

  @Test
  fun `timer with two callbacks with different return types should return same types`() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val apiTimerProvider =
      TimerFactoryImpl(
        meterRegistry = meterRegistry,
        name = "request.timer",
        description = "API request timer",
        commonTags = listOf(Tag("version", "v1.0")),
      )

    val result1 = apiTimerProvider.create(tags = listOf(Tag("method", "eth_blockNumber"))).captureTime {
      Thread.sleep(10)
      101
    }
    Assertions.assertThat(result1).isEqualTo(101)

    val result2 = apiTimerProvider.create(tags = listOf(Tag("method", "eth_status"))).captureTime {
      Thread.sleep(10)
      "synced"
    }
    Assertions.assertThat(result2).isEqualTo("synced")

    Assertions.assertThat(meterRegistry.meters.size).isEqualTo(2)
    meterRegistry.meters.forEach { meter ->
      Assertions.assertThat(meter.id.type).isEqualTo(Meter.Type.TIMER)
      Assertions.assertThat(meter.id.name).isEqualTo("request.timer")
      Assertions.assertThat(meter.id.description).isEqualTo("API request timer")
      Assertions.assertThat(meter.id.tags.size).isEqualTo(2)
      Assertions.assertThat(meter.id.tags.map { it.key }).containsExactlyInAnyOrder("version", "method")
      Assertions.assertThat((meter as Timer).totalTime(TimeUnit.MILLISECONDS)).isBetween(10.0, 100.0)
    }
    Assertions.assertThat(meterRegistry.find("request.timer").tags("method", "eth_blockNumber").timer())
      .isNotNull
    Assertions.assertThat(meterRegistry.find("request.timer").tags("method", "eth_status").timer())
      .isNotNull
    Assertions.assertThat(meterRegistry.meters.flatMap { it.id.tags }.map { it.value })
      .contains("eth_blockNumber", "eth_status")
  }

  @Test
  fun `timer with callback that throws should measure time and also throw the exception`() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val apiTimerProvider =
      TimerFactoryImpl(
        meterRegistry = meterRegistry,
        name = "request.timer",
        description = "API request timer",
        commonTags = listOf(Tag("version", "v1.0")),
      )

    val result1 = apiTimerProvider.create(tags = listOf(Tag("method", "eth_blockNumber"))).captureTime {
      101
    }
    Assertions.assertThat(result1).isEqualTo(101)

    val exception = assertThrows<IllegalStateException> {
      apiTimerProvider.create(tags = listOf(Tag("method", "eth_status"))).captureTime {
        throw IllegalStateException("sync_error")
      }
    }
    Assertions.assertThat(exception.message).isEqualTo("sync_error")

    Assertions.assertThat(meterRegistry.meters.size).isEqualTo(2)
    meterRegistry.meters.forEach { meter ->
      Assertions.assertThat(meter.id.type).isEqualTo(Meter.Type.TIMER)
      Assertions.assertThat(meter.id.name).isEqualTo("request.timer")
      Assertions.assertThat(meter.id.description).isEqualTo("API request timer")
      Assertions.assertThat(meter.id.tags.size).isEqualTo(2)
      Assertions.assertThat(meter.id.tags.map { it.key }).containsExactlyInAnyOrder("version", "method")
    }
    Assertions.assertThat(meterRegistry.find("request.timer").tags("method", "eth_blockNumber").timer())
      .isNotNull
    Assertions.assertThat(meterRegistry.find("request.timer").tags("method", "eth_status").timer())
      .isNotNull
    Assertions.assertThat(meterRegistry.meters.flatMap { it.id.tags }.map { it.value })
      .contains("eth_blockNumber", "eth_status")
  }

  @Test
  fun `timer with future should return the correct value and exception if thrown`() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val apiTimerProvider =
      TimerFactoryImpl(
        meterRegistry = meterRegistry,
        name = "request.timer",
        description = "API request timer",
        commonTags = listOf(Tag("version", "v1.0")),
        clock = MockClock(),
      )

    val result1 = apiTimerProvider.create(listOf(Tag("method", "eth_blockNumber")))
      .captureTime(SafeFuture.completedFuture(101))
    Assertions.assertThat(result1.get()).isEqualTo(101)

    val result2 = apiTimerProvider.create(listOf(Tag("method", "eth_status")))
      .captureTime(SafeFuture.failedFuture<String>(IllegalStateException("sync_error")))
    val exception = assertThrows<ExecutionException> { result2.get() }
    Assertions.assertThat(exception.cause).isInstanceOf(IllegalStateException::class.java)
    Assertions.assertThat(exception.cause?.message).isEqualTo("sync_error")

    Assertions.assertThat(meterRegistry.meters.size).isEqualTo(2)
    meterRegistry.meters.forEach { meter ->
      Assertions.assertThat(meter.id.type).isEqualTo(Meter.Type.TIMER)
      Assertions.assertThat(meter.id.name).isEqualTo("request.timer")
      Assertions.assertThat(meter.id.description).isEqualTo("API request timer")
      Assertions.assertThat(meter.id.tags.size).isEqualTo(2)
      Assertions.assertThat(meter.id.tags.map { it.key }).containsExactlyInAnyOrder("version", "method")
    }
    Assertions.assertThat(meterRegistry.find("request.timer").tags("method", "eth_blockNumber").timer())
      .isNotNull
    Assertions.assertThat(meterRegistry.find("request.timer").tags("method", "eth_status").timer())
      .isNotNull
    Assertions.assertThat(meterRegistry.meters.flatMap { it.id.tags }.map { it.value })
      .contains("eth_blockNumber", "eth_status")
  }
}
