package lineth.coordinator.config.v2.toml

import lineth.config.docs.ConfigDoc
import lineth.config.docs.ConfigSection
import java.net.URL
import kotlin.time.Duration.Companion.seconds

data class DefaultsToml(
  @param:ConfigDoc(
    description = "Default L1 execution-layer JSON-RPC endpoint used by services that do not " +
      "override it.",
    example = "http://l1-el-node:8545",
  )
  val l1Endpoint: URL? = null,
  @param:ConfigDoc(
    description = "Default L2 JSON-RPC endpoint used by services that do not override it.",
    example = "http://sequencer:8545",
  )
  val l2Endpoint: URL? = null,
  @param:ConfigSection("Default retry policy for L1 requests, reused by services that do not override it.")
  val l1RequestRetries: RequestRetriesToml =
    RequestRetriesToml.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  @param:ConfigSection("Default retry policy for L2 requests, reused by services that do not override it.")
  val l2RequestRetries: RequestRetriesToml =
    RequestRetriesToml.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
)
