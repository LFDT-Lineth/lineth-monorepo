package net.consensys.linea.metrics.micrometer

import io.micrometer.core.instrument.MeterRegistry
import io.micrometer.core.instrument.MockClock
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Future
import net.consensys.linea.async.get
import net.consensys.linea.async.toCompletableFuture
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.async.toVertxFuture
import net.consensys.linea.metrics.Tag
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.time.Duration
import java.util.concurrent.TimeUnit

internal class DynamicTagTimerTest {
  @Test
  fun itExtractsValueAndDoesntAlterResult_callback() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val testClock = MockClock()
    val result =
      DynamicTagTimerImpl<String>(
        meterRegistry = meterRegistry,
        name = "request.counter",
        description = "API request counter",
        clock = testClock,
        extractor = { listOf(Tag(key = "method", value = "eth_blockNumber")) },
        extractorOnError = { listOf(Tag(key = "method", value = "eth_blockNumber_failure")) },
        commonTags = emptyList(),
      ).captureTime {
        testClock.add(Duration.ofSeconds(1))
        "measured_callback_result"
      }
    assertThat(result).isEqualTo("measured_callback_result")
    val createdMeter = meterRegistry["request.counter"].timer()

    assertThat(createdMeter.count()).isEqualTo(1)
    assertThat(createdMeter.totalTime(TimeUnit.SECONDS)).isEqualTo(1.0)
    assertThat(createdMeter.id.description).isEqualTo("API request counter")
    assertThat(createdMeter.id.tags.size).isEqualTo(1)
    assertThat(createdMeter.id.tags[0].key).isEqualTo("method")
    assertThat(createdMeter.id.tags[0].value).isEqualTo("eth_blockNumber")
    assertThat(createdMeter.totalTime(TimeUnit.SECONDS)).isEqualTo(1.0)
  }

  @Test
  fun itExtractsValueAndDoesntAlterResult_CallbackException() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val testClock = MockClock()
    val result = assertThrows<Exception> {
      DynamicTagTimerImpl<String>(
        meterRegistry = meterRegistry,
        name = "request.counter",
        description = "API request counter",
        clock = testClock,
        extractor = { listOf(Tag(key = "method", value = "eth_blockNumber")) },
        extractorOnError = { listOf(Tag(key = "method", value = "eth_blockNumber_failure")) },
        commonTags = emptyList(),
      ).captureTime {
        testClock.add(Duration.ofSeconds(1))
        throw Exception("measured_callback_error")
      }
    }
    assertThat(result.message).isEqualTo("measured_callback_error")
    val createdMeter = meterRegistry["request.counter"].timer()

    assertThat(createdMeter.count()).isEqualTo(1)
    assertThat(createdMeter.totalTime(TimeUnit.SECONDS)).isEqualTo(1.0)
    assertThat(createdMeter.id.description).isEqualTo("API request counter")
    assertThat(createdMeter.id.tags.size).isEqualTo(1)
    assertThat(createdMeter.id.tags[0].key).isEqualTo("method")
    assertThat(createdMeter.id.tags[0].value).isEqualTo("eth_blockNumber_failure")
    assertThat(createdMeter.totalTime(TimeUnit.SECONDS)).isEqualTo(1.0)
  }

  @Test
  fun itExtractsValue_FutureSuccess() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val testClock = MockClock()
    val future: SafeFuture<String> = SafeFuture()
    val result =
      DynamicTagTimerImpl<String>(
        meterRegistry = meterRegistry,
        name = "request.counter",
        description = "API request counter",
        clock = testClock,
        extractor = { listOf(Tag(key = "method", value = "eth_blockNumber")) },
        extractorOnError = { listOf(Tag(key = "method", value = "eth_blockNumber_failure")) },
        commonTags = emptyList(),
      )
        .captureTime(Future.succeededFuture("measured_callback_result").toCompletableFuture())
        .toVertxFuture()
    future.complete("measured_callback_result")

    assertThat(result.get()).isEqualTo("measured_callback_result")

    val createdMeter = meterRegistry["request.counter"].timer()
    assertThat(createdMeter.count()).isEqualTo(1)
    assertThat(createdMeter.id.description).isEqualTo("API request counter")
    assertThat(createdMeter.id.tags.size).isEqualTo(1)
    assertThat(createdMeter.id.tags[0].key).isEqualTo("method")
    assertThat(createdMeter.id.tags[0].value).isEqualTo("eth_blockNumber")
  }

  @Test
  fun itExtractsValue_FutureFailure() {
    val meterRegistry: MeterRegistry = SimpleMeterRegistry()
    val testClock = MockClock()
    val result =
      DynamicTagTimerImpl<String>(
        meterRegistry = meterRegistry,
        name = "request.counter",
        description = "API request counter",
        clock = testClock,
        extractor = { listOf(Tag(key = "method", value = "eth_blockNumber")) },
        extractorOnError = { listOf(Tag(key = "method", value = "eth_blockNumber_failure")) },
        commonTags = emptyList(),
      )
        .captureTime(SafeFuture.failedFuture(Exception("measured_callback_error")))
        .toSafeFuture()

    result.finish { error -> assertThat(error.message).isEqualTo("measured_callback_error") }

    val createdMeter = meterRegistry["request.counter"].timer()
    assertThat(createdMeter.count()).isEqualTo(1)
    assertThat(createdMeter.id.description).isEqualTo("API request counter")
    assertThat(createdMeter.id.tags.size).isEqualTo(1)
    assertThat(createdMeter.id.tags[0].key).isEqualTo("method")
    assertThat(createdMeter.id.tags[0].value).isEqualTo("eth_blockNumber_failure")
  }
}
