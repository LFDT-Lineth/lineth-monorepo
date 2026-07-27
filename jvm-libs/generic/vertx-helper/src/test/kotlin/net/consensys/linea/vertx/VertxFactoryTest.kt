package net.consensys.linea.vertx

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds

class VertxFactoryTest {
  @Test
  fun `configures worker execution time in milliseconds`() {
    val options = VertxFactory.createVertxOptions(
      maxWorkerExecuteTime = 123.milliseconds,
      jvmMetricsEnabled = false,
      prometheusMetricsEnabled = false,
    )

    assertThat(options.maxWorkerExecuteTime).isEqualTo(123L)
    assertThat(options.maxWorkerExecuteTimeUnit).isEqualTo(TimeUnit.MILLISECONDS)
  }
}
