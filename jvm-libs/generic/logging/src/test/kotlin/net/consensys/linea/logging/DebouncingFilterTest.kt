package net.consensys.linea.logging

import org.apache.logging.log4j.Level
import org.apache.logging.log4j.core.Filter
import org.apache.logging.log4j.core.impl.Log4jLogEvent
import org.apache.logging.log4j.message.SimpleMessage
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.RepeatedTest
import org.junit.jupiter.api.Test
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.time.Duration.Companion.milliseconds

class DebouncingFilterTest {
  private val debounceTime = 5.milliseconds
  private lateinit var debouncer: DebouncingFilter

  @BeforeEach
  fun beforeEach() {
    debouncer = DebouncingFilter(
      debounceTime = debounceTime,
      maxCacheCapacity = 3,
    )
  }

  @Test
  fun debouncesRepeatedLogs() {
    val loggedString = "AAAAAA!"
    val loggedLevel = Level.WARN
    val eventsBuilder = Log4jLogEvent().asBuilder()
      .setMessage(SimpleMessage(loggedString))
      .setLevel(loggedLevel)
    assertThat(debouncer.filter(eventsBuilder.setTimeMillis(12).build())).isEqualTo(Filter.Result.ACCEPT)
    assertThat(debouncer.filter(eventsBuilder.setTimeMillis(13).build())).isEqualTo(Filter.Result.DENY)
    assertThat(debouncer.filter(eventsBuilder.setTimeMillis(14).build())).isEqualTo(Filter.Result.DENY)
    assertThat(debouncer.filter(eventsBuilder.setTimeMillis(15).build())).isEqualTo(Filter.Result.DENY)
    assertThat(debouncer.filter(eventsBuilder.setTimeMillis(16).build())).isEqualTo(Filter.Result.DENY)
  }

  @Test
  fun doesntDebounceAfterDebounceTime() {
    val loggedString = "AAAAAA!"
    val eventsBuilder = createLogEventBuilderWithString(loggedString)
    val initialMessageTime = 12L
    assertThat(
      debouncer.filter(
        eventsBuilder.setTimeMillis(initialMessageTime).build(),
      ),
    ).isEqualTo(Filter.Result.ACCEPT)
    assertThat(
      debouncer.filter(
        eventsBuilder.setTimeMillis(initialMessageTime + debounceTime.inWholeMilliseconds + 1).build(),
      ),
    ).isEqualTo(Filter.Result.ACCEPT)
  }

  @Test
  fun doesntDebounceWithDifferentLogLevels() {
    val loggedString = "AAAAAA!"
    val loggedLevel = Level.ERROR
    val eventsBuilder = Log4jLogEvent().asBuilder()
      .setMessage(SimpleMessage(loggedString))
      .setLevel(loggedLevel)
    val loggedLevel2 = Level.DEBUG
    val eventsBuilder2 = Log4jLogEvent().asBuilder()
      .setMessage(SimpleMessage(loggedString))
      .setLevel(loggedLevel2)
    assertThat(debouncer.filter(eventsBuilder.setTimeMillis(12).build())).isEqualTo(Filter.Result.ACCEPT)
    assertThat(debouncer.filter(eventsBuilder.setTimeMillis(13).build())).isEqualTo(Filter.Result.DENY)
    assertThat(debouncer.filter(eventsBuilder2.setTimeMillis(14).build())).isEqualTo(Filter.Result.ACCEPT)
    assertThat(debouncer.filter(eventsBuilder2.setTimeMillis(15).build())).isEqualTo(Filter.Result.DENY)
  }

  @Test
  fun debounceCacheIsLimited() {
    val theSameMessageTime = 12L
    assertThat(
      debouncer.filter(
        createLogEventBuilderWithString("Message 1").setTimeMillis(theSameMessageTime).build(),
      ),
    ).isEqualTo(Filter.Result.ACCEPT)
    assertThat(
      debouncer.filter(
        createLogEventBuilderWithString("Message 2").setTimeMillis(theSameMessageTime).build(),
      ),
    ).isEqualTo(Filter.Result.ACCEPT)
    assertThat(
      debouncer.filter(
        createLogEventBuilderWithString("Message 3").setTimeMillis(theSameMessageTime).build(),
      ),
    ).isEqualTo(Filter.Result.ACCEPT)
    assertThat(
      debouncer.filter(
        createLogEventBuilderWithString("Message 3").setTimeMillis(theSameMessageTime).build(),
      ),
    ).isEqualTo(Filter.Result.DENY)
    // Message 4 pushes Message 1 out of the LRU cache
    assertThat(
      debouncer.filter(
        createLogEventBuilderWithString("Message 4").setTimeMillis(theSameMessageTime).build(),
      ),
    ).isEqualTo(Filter.Result.ACCEPT)
    assertThat(
      debouncer.filter(
        createLogEventBuilderWithString("Message 1").setTimeMillis(theSameMessageTime).build(),
      ),
    ).isEqualTo(Filter.Result.ACCEPT)
  }

  @RepeatedTest(100)
  fun concurrencyNegativeTest() {
    val theSameMessage = createLogEventBuilderWithString("Message 1").setTimeMillis(12L).build()
    val resultsSink = CopyOnWriteArrayList<Filter.Result>()

    val threads = (1..10).map {
      Thread {
        resultsSink.add(debouncer.filter(theSameMessage))
      }
    }
    threads.forEach(Thread::start)
    threads.forEach {
      it.join()
    }
    assertThat(resultsSink.size).isEqualTo(10)
    assertThat(resultsSink.filter { it == Filter.Result.ACCEPT }.size).isEqualTo(1)
  }

  @RepeatedTest(100)
  fun concurrencyPositiveTest() {
    val resultsSink = CopyOnWriteArrayList<Filter.Result>()

    val threads = (1..10).map {
      Thread {
        resultsSink.add(debouncer.filter(createLogEventBuilderWithString("Message $it").setTimeMillis(12L).build()))
      }
    }
    threads.forEach(Thread::start)
    threads.forEach {
      it.join()
    }
    assertThat(resultsSink.size).isEqualTo(10)
    assertThat(resultsSink.filter { it == Filter.Result.ACCEPT }.size).isEqualTo(10)
  }

  private fun createLogEventBuilderWithString(s: String): Log4jLogEvent.Builder {
    return Log4jLogEvent().asBuilder()
      .setMessage(SimpleMessage(s))
  }
}
