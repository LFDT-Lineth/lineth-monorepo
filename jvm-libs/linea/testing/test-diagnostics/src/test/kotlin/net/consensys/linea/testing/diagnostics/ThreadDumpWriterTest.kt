package net.consensys.linea.testing.diagnostics

import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

class ThreadDumpWriterTest {
  @Test
  fun `render reports the JVM figures and every live thread with its stack`() {
    val release = CountDownLatch(1)
    val parked = Thread({ release.await() }, "diagnostics-test-parked-thread").apply {
      isDaemon = true
      start()
    }
    try {
      await().atMost(10, TimeUnit.SECONDS).until { parked.state == Thread.State.WAITING }

      val dump = ThreadDumpWriter.render(listOf("trigger: unit test"))

      assertThat(dump).contains(
        "trigger: unit test",
        "availableProcessors=${Runtime.getRuntime().availableProcessors()}",
        "=== Top 15 threads by CPU time",
        "=== All threads ===",
        "\"diagnostics-test-parked-thread\"",
        "state=WAITING",
        "CountDownLatch",
      )
    } finally {
      release.countDown()
      parked.join(TimeUnit.SECONDS.toMillis(10))
    }
  }

  @Test
  fun `write creates the missing parent directories`(@TempDir dir: Path) {
    val file = dir.resolve("a/b/thread-dump.txt")

    ThreadDumpWriter.write(file, listOf("trigger: unit test"))

    assertThat(Files.readString(file)).contains("trigger: unit test", "=== Test JVM snapshot ===")
  }
}
