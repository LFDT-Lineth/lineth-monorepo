package linea.coordinator.config.v2

import linea.domain.BlockParameter
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class L1FinalizationMonitorConfig(
  val l1Endpoint: URL,
  val l2Endpoint: URL,
  val l1PollingInterval: Duration = 6.seconds,
  val l1QueryBlockTag: BlockParameter.Tag = BlockParameter.Tag.FINALIZED,
)
