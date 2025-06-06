package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.TracesConfig
import java.net.URL
import kotlin.time.Duration.Companion.seconds

data class TracesToml(
  val expectedTracesApiVersion: String,
  val counters: ClientApiConfigToml,
  val conflation: ClientApiConfigToml,
  val switchBlockNumberInclusive: UInt? = null,
  val new: TracesToml? = null,
) {
  data class ClientApiConfigToml(
    val endpoints: List<URL>,
    val requestLimitPerEndpoint: UInt = UInt.MAX_VALUE,
    val requestRetries: RequestRetriesToml = RequestRetriesToml.endlessRetry(
      backoffDelay = 1.seconds,
      failuresWarningThreshold = 3u,
    ),
  ) {
    override fun toString(): String {
      return "ClientApiConfigToml(" +
        "endpoints=$endpoints, " +
        "requestLimitPerEndpoint=$requestLimitPerEndpoint, " +
        "requestRetries=$requestRetries" +
        ")"
    }
  }

  fun reified(): TracesConfig {
    return TracesConfig(
      expectedTracesApiVersion = expectedTracesApiVersion,
      counters = TracesConfig.ClientApiConfig(
        endpoints = counters.endpoints,
        requestLimitPerEndpoint = counters.requestLimitPerEndpoint,
        requestRetries = counters.requestRetries.asDomain,
      ),
      conflation = TracesConfig.ClientApiConfig(
        endpoints = conflation.endpoints,
        requestLimitPerEndpoint = conflation.requestLimitPerEndpoint,
        requestRetries = conflation.requestRetries.asDomain,
      ),
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
