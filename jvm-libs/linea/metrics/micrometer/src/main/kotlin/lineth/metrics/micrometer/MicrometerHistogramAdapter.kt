package lineth.metrics.micrometer

import io.micrometer.core.instrument.DistributionSummary
import lineth.metrics.Histogram

class MicrometerHistogramAdapter(private val adapter: DistributionSummary) : Histogram {
  override fun record(data: Double) {
    adapter.record(data)
  }
}
