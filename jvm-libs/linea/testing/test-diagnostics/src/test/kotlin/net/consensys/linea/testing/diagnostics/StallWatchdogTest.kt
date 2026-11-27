package net.consensys.linea.testing.diagnostics

import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

class StallWatchdogTest {
  private val events = CopyOnWriteArrayList<StallWatchdog.StallEvent>()
  private var watchdog: StallWatchdog? = null

  @AfterEach
  fun tearDown() {
    watchdog?.close()
  }

  private fun watchdog(thresholdsMs: List<Long>, onStall: (StallWatchdog.StallEvent) -> Unit = { events += it }) =
    StallWatchdog(thresholdsMs, onStall).also { watchdog = it }

  @Test
  fun `fires once per threshold while a task is still running`() {
    val watchdog = watchdog(listOf(20L, 40L))

    watchdog.started("task-a")

    await().atMost(10, TimeUnit.SECONDS).untilAsserted {
      assertThat(events.map { it.thresholdMs }).containsExactly(20L, 40L)
    }
    assertThat(events.map { it.key }.distinct()).containsExactly("task-a")
    assertThat(events.map { it.elapsedMs }).allSatisfy { assertThat(it).isGreaterThanOrEqualTo(19L) }
  }

  @Test
  fun `finishing a task cancels its pending checks`() {
    // far in the future: the checks can only go away by being cancelled
    val watchdog = watchdog(listOf(600_000L, 1_200_000L))

    watchdog.started("task-a")
    assertThat(watchdog.scheduledCheckCount()).isEqualTo(2)

    watchdog.finished("task-a")
    assertThat(watchdog.scheduledCheckCount()).isZero()
    assertThat(events).isEmpty()
  }

  @Test
  fun `tracks the tasks currently running`() {
    val watchdog = watchdog(listOf(600_000L))

    watchdog.started("task-a")
    watchdog.started("task-b")
    assertThat(watchdog.runningTasks().keys).containsExactlyInAnyOrder("task-a", "task-b")

    watchdog.finished("task-a")
    assertThat(watchdog.runningTasks().keys).containsExactly("task-b")
  }

  @Test
  fun `the event lists every task running when it fires`() {
    val watchdog = watchdog(listOf(30L))

    watchdog.started("task-a")
    watchdog.started("task-b")

    await().atMost(10, TimeUnit.SECONDS).untilAsserted {
      assertThat(events).isNotEmpty()
    }
    // task-b may have been started just after task-a's check, but task-a is always part of its own event
    assertThat(events.first { it.key == "task-a" }.running).containsKey("task-a")
  }

  @Test
  fun `a failing callback does not stop the following checks`() {
    val calls = AtomicInteger()
    val watchdog = watchdog(listOf(20L, 40L)) {
      calls.incrementAndGet()
      throw IllegalStateException("callback failure expected by the test")
    }

    watchdog.started("task-a")

    await().atMost(10, TimeUnit.SECONDS).untilAsserted {
      assertThat(calls.get()).isEqualTo(2)
    }
  }
}
