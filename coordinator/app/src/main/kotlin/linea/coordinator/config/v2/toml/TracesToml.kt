package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.TracesConfig
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class TracesToml(
  val expectedTracesApiVersion: String,
  val endpoints: List<URL>? = null,
  val requestLimitPerEndpoint: UInt = UInt.MAX_VALUE,
  val requestTimeout: Duration? = null,
  val requestRetries: RequestRetriesToml =
    RequestRetriesToml.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  val counters: ClientApiConfigToml? = null,
  val conflation: ClientApiConfigToml? = null,
  val ignoreTracesGeneratorErrors: Boolean = false,
  val switchBlockNumberInclusive: UInt? = null,
  val new: TracesToml? = null,
) {
  init {
    require(endpoints != null || (counters?.endpoints !== null && conflation?.endpoints !== null)) {
      "either traces.endpoints " +
        "or traces.counters.endpoints and traces.conflation.endpoints must be set"
    }
  }

  data class ClientApiConfigToml(
    val endpoints: List<URL>? = null,
    val requestLimitPerEndpoint: UInt? = null,
    val requestTimeout: Duration? = null,
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
      endpoints = (config?.endpoints ?: endpoints)!!,
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
          endpoints = endpoints!!,
          requestLimitPerEndpoint = requestLimitPerEndpoint,
          requestTimeout = requestTimeout,
          requestRetries = requestRetries.asDomain,
        )
      }

    return TracesConfig(
      expectedTracesApiVersion = expectedTracesApiVersion,
      common = common,
      counters = if (common == null) reifiedWithCommonDefaults(this.counters) else null,
      conflation = if (common == null) reifiedWithCommonDefaults(this.conflation) else null,
      ignoreTracesGeneratorErrors = ignoreTracesGeneratorErrors,
      /*
      switchBlockNumberInclusive = switchBlockNumberInclusive,
      new = new?.let { newTracesConfig ->
        TracesConfig(
          expectedTracesApiVersion = newTracesConfig.expectedTracesApiVersion,
          counters = TracesConfig.ClientApiConfig(
            endpoints = newTracesConfig.counters.endpoints,
            requestLimitPerEndpoint = newTracesConfig.counters.requestLimitPerEndpoint,
            requestRetries = newTracesConfig.counters.requestRetries.asDomain
          ),
          conflation = TracesConfig.ClientApiConfig(
            endpoints = newTracesConfig.conflation.endpoints,
            requestLimitPerEndpoint = newTracesConfig.conflation.requestLimitPerEndpoint,
            requestRetries = newTracesConfig.conflation.requestRetries.asDomain
          ),
          switchBlockNumberInclusive = null,
          new = null
        )
      }
       */
    )
  }
}
