package net.consensys.zkevm.ethereum.coordination

import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import java.util.concurrent.atomic.AtomicInteger
import java.util.function.Consumer

class EventDispatcherTest {

  @Test
  fun dispatcherContinuesAfterFailedAttempt() {
    val number1 = AtomicInteger(0)
    val number2 = AtomicInteger(0)

    val mappedConsumers: Map<Consumer<AtomicInteger>, String> = mapOf(
      Consumer<AtomicInteger> { atomicInteger ->
        atomicInteger.incrementAndGet()
      } to "Consumer counter 1",
      Consumer<AtomicInteger> {
        throw RuntimeException("Test error")
      } to "Consumer error",
      Consumer<AtomicInteger> { atomicInteger ->
        atomicInteger.incrementAndGet()
      } to "Consumer counter 2",
    )

    val eventDispatcher = EventDispatcher(mappedConsumers)

    eventDispatcher.accept(number1)
    assertThat(number1.get()).isEqualTo(2)
    assertThat(number2.get()).isEqualTo(0)

    eventDispatcher.accept(number2)
    assertThat(number1.get()).isEqualTo(2)
    assertThat(number2.get()).isEqualTo(2)
  }

  @Test
  fun dispatcherErrorPrintsCorrectly() {
    val consumerName = "Mock Consumer"
    val exceptionMessage = "Test error"

    val mappedConsumers: Map<Consumer<Any>, String> = mapOf(
      Consumer<Any> {
        throw RuntimeException(exceptionMessage)
      } to consumerName,
    )

    val log = Mockito.spy(LogManager.getLogger(EventDispatcher::class.java))
    val eventDispatcher = EventDispatcher(mappedConsumers, log)

    val event = { }
    eventDispatcher.accept(event)

    await()
      .untilAsserted {
        verify(log, times(1)).warn(
          eq("Failed to consume event: consumer={} event={} errorMessage={}"),
          eq(consumerName),
          eq(event),
          eq(exceptionMessage),
          any(),
        )
      }
  }
}
