package lineth.metrics.micrometer

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.util.concurrent.atomic.AtomicInteger

class GaugeAggregatorTest {

  @Test
  fun `should aggregate multiple counters`() {
    val counterA = AtomicInteger(1)
    val counterB = AtomicInteger(2)
    val aggregator = GaugeAggregator()

    aggregator.addReporter(counterA::get)
    aggregator.addReporter(counterB::get)

    assertThat(aggregator.get()).isEqualTo(3L)

    counterB.set(10)
    assertThat(aggregator.get()).isEqualTo(11L)
  }
}
