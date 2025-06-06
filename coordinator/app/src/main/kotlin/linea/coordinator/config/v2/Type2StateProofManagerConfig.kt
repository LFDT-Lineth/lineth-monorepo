package linea.coordinator.config.v2

import linea.domain.BlockParameter
import linea.domain.RetryConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class Type2StateProofManagerConfig(
  override val disabled: Boolean = false,
  val endpoints: List<URL>,
  val requestRetries: RetryConfig = RetryConfig.endlessRetry(
    backoffDelay = 1.seconds,
    failuresWarningThreshold = 3u,
  ),
  val l1QueryBlockTag: BlockParameter.Tag = BlockParameter.Tag.FINALIZED,
  val l1PollingInterval: Duration = 6.seconds,
) : FeatureToggle
