package lineth.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import lineth.coordinator.config.v2.TracesConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class TracesToml(
  @param:ConfigDoc(
    description = "Traces generator endpoints shared by the counters and conflation clients. " +
      "Required unless both counters.endpoints and conflation.endpoints are set.",
    example = "[\"http://traces-node:8545\"]",
  )
  val endpoints: List<URL>? = null,
  @param:ConfigDoc(
    description = "Maximum number of concurrent in-flight requests per traces endpoint.",
    default = "4294967295",
  )
  val requestLimitPerEndpoint: UInt = UInt.MAX_VALUE,
  @param:ConfigDoc(
    description = "Timeout for each traces generator request. Omit to disable the timeout.",
    example = "PT30S",
  )
  val requestTimeout: Duration? = null,
  @param:ConfigSection("Retry policy for traces generator requests.")
  val requestRetries: RequestRetriesToml =
    RequestRetriesToml.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  @param:ConfigSection("Overrides for the traces counters client; falls back to the shared traces settings.")
  val counters: ClientApiConfigToml? = null,
  @param:ConfigSection("Overrides for the traces conflation client; falls back to the shared traces settings.")
  val conflation: ClientApiConfigToml? = null,
  @param:ConfigDoc(
    description = "Whether to continue despite traces generator errors instead of failing.",
    default = "false",
  )
  val ignoreTracesGeneratorErrors: Boolean = false,
  @param:ConfigDoc(
    description = "Inclusive L2 block number at which to switch from this traces config to the `new` one.",
    example = "1000000",
  )
  val switchBlockNumberInclusive: UInt? = null,
  @param:ConfigSection("Next traces client config to switch over to at the configured switch block.")
  val new: TracesToml? = null,
) {
  init {
    require(endpoints != null || (counters?.endpoints !== null && conflation?.endpoints !== null)) {
      "either traces.endpoints " +
        "or traces.counters.endpoints and traces.conflation.endpoints must be set"
    }
  }

  data class ClientApiConfigToml(
    @param:ConfigDoc(
      description = "Endpoints for this specific traces client. Falls back to traces.endpoints when omitted.",
      example = "[\"http://traces-node:8545\"]",
    )
    val endpoints: List<URL>? = null,
    @param:ConfigDoc(
      description = "Per-endpoint concurrent request limit for this client. Falls back to the shared value.",
      example = "4",
    )
    val requestLimitPerEndpoint: UInt? = null,
    @param:ConfigDoc(
      description = "Request timeout for this client. Falls back to the shared value.",
      example = "PT30S",
    )
    val requestTimeout: Duration? = null,
    @param:ConfigSection("Retry policy for this client; falls back to the shared traces retry policy.")
    val requestRetries: RequestRetriesToml? = null,
  ) {
    override fun toString(): String {
      return "ClientApiConfigToml(" +
        "endpoints=$endpoints, " +
        "requestLimitPerEndpoint=$requestLimitPerEndpoint, " +
        "requestTimeout=$requestTimeout, " +
        "requestRetries=$requestRetries" +
        ")"
    }
  }

  private fun reifiedWithCommonDefaults(config: ClientApiConfigToml?): TracesConfig.ClientApiConfig {
    return TracesConfig.ClientApiConfig(
      endpoints = requireNotNull(config?.endpoints ?: endpoints) {
        "endpoints must be set either in the specific config or in the common traces config"
      },
      requestLimitPerEndpoint = config?.requestLimitPerEndpoint ?: requestLimitPerEndpoint,
      requestTimeout = config?.requestTimeout ?: requestTimeout,
      requestRetries = config?.requestRetries?.asDomain ?: requestRetries.asDomain,
    )
  }

  fun reified(): TracesConfig {
    val common =
      if (counters !== null || conflation != null) {
        // when specific counters or conflation are set, common must be null
        null
      } else {
        TracesConfig.ClientApiConfig(
          endpoints = requireNotNull(endpoints) {
            "traces.endpoints must be set when counters and conflation specific endpoints are not configured"
          },
          requestLimitPerEndpoint = requestLimitPerEndpoint,
          requestTimeout = requestTimeout,
          requestRetries = requestRetries.asDomain,
        )
      }

    return TracesConfig(
      common = common,
      counters = if (common == null) reifiedWithCommonDefaults(this.counters) else null,
      conflation = if (common == null) reifiedWithCommonDefaults(this.conflation) else null,
      ignoreTracesGeneratorErrors = ignoreTracesGeneratorErrors,
    )
  }
}
