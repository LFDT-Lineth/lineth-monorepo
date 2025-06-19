package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.RequestRetriesToml
import linea.coordinator.config.v2.toml.TracesToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.net.URI
import kotlin.time.Duration.Companion.seconds

class TracesParsingTest {
  companion object {
    val toml = """
    [traces]
    expected-traces-api-version = "1.2.0"
    [traces.counters]
    endpoints = ["http://traces-api-1:8080/"]
    request-limit-per-endpoint = 20
    [traces.counters.request-retries]
    max-retries = 40
    backoff-delay = "PT1S"
    failures-warning-threshold = 2

    [traces.conflation]
    endpoints = ["http://traces-api-2:8080/"]
    request-limit-per-endpoint = 2
    request-timeout = "PT60S"
    [traces.conflation.request-retries]
    max-retries = 30
    backoff-delay = "PT3S"
    failures-warning-threshold = 4

    [traces.new]
    switch-block-number-inclusive=1_000
    expected-traces-api-version = "2.0.0"
    [traces.new.counters]
    endpoints = ["http://traces-api-v2-11:8080/", "http://traces-api-v2-12:8080/"]
    request-limit-per-endpoint = 200
    [traces.new.counters.request-retries]
    max-retries = 4
    backoff-delay = "PT1S"
    failures-warning-threshold = 2

    [traces.new.conflation]
    endpoints = ["http://traces-api-v2-21:8080/", "http://traces-api-v2-22:8080/"]
    request-limit-per-endpoint = 5
    [traces.new.conflation.request-retries]
    max-retries = 55
    backoff-delay = "PT50S"
    failures-warning-threshold = 50
    """.trimIndent()

    val config = TracesToml(
      expectedTracesApiVersion = "1.2.0",
      counters = TracesToml.ClientApiConfigToml(
        endpoints = listOf(URI.create("http://traces-api-1:8080/").toURL()),
        requestLimitPerEndpoint = 20u,
        requestRetries = RequestRetriesToml(
          maxRetries = 40u,
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 2u,
        ),
      ),
      conflation = TracesToml.ClientApiConfigToml(
        endpoints = listOf(URI.create("http://traces-api-2:8080/").toURL()),
        requestLimitPerEndpoint = 2u,
        requestTimeout = 60.seconds,
        requestRetries = RequestRetriesToml(
          maxRetries = 30u,
          backoffDelay = 3.seconds,
          failuresWarningThreshold = 4u,
        ),
      ),
      new = TracesToml(
        expectedTracesApiVersion = "2.0.0",
        switchBlockNumberInclusive = 1_000u,
        counters = TracesToml.ClientApiConfigToml(
          endpoints = listOf("http://traces-api-v2-11:8080/", "http://traces-api-v2-12:8080/").map { it.toURL() },
          requestLimitPerEndpoint = 200u,
          requestRetries = RequestRetriesToml(
            maxRetries = 4u,
            backoffDelay = 1.seconds,
            failuresWarningThreshold = 2u,
          ),
        ),
        conflation = TracesToml.ClientApiConfigToml(
          endpoints = listOf("http://traces-api-v2-21:8080/", "http://traces-api-v2-22:8080/").map { it.toURL() },
          requestLimitPerEndpoint = 5u,
          requestRetries = RequestRetriesToml(
            maxRetries = 55u,
            backoffDelay = 50.seconds,
            failuresWarningThreshold = 50u,
          ),
        ),
      ),
    )

    val tomlMinimal = """
    [traces]
    expected-traces-api-version = "1.2.0"
    [traces.counters]
    endpoints = ["http://traces-api-1:8080/"]
    [traces.conflation]
    endpoints = ["http://traces-api-2:8080/"]
    """.trimIndent()

    val configMinimal = TracesToml(
      expectedTracesApiVersion = "1.2.0",
      counters = TracesToml.ClientApiConfigToml(
        endpoints = listOf(URI.create("http://traces-api-1:8080/").toURL()),
        requestLimitPerEndpoint = UInt.MAX_VALUE,
        requestTimeout = null,
        requestRetries = RequestRetriesToml(
          maxRetries = null,
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 3u,
        ),
      ),
      conflation = TracesToml.ClientApiConfigToml(
        endpoints = listOf(URI.create("http://traces-api-2:8080/").toURL()),
        requestLimitPerEndpoint = UInt.MAX_VALUE,
        requestTimeout = null,
        requestRetries = RequestRetriesToml(
          maxRetries = null,
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 3u,
        ),
      ),
    )
  }

  data class WrapperConfig(
    val traces: TracesToml,
  )

  @Test
  fun `should parse ClientApiConfigToml config`() {
    assertThat(
      parseConfig<TracesToml.ClientApiConfigToml>(
        """
    endpoints = ["http://traces-api-1:8080/", "http://traces-api-2:8080/"]
    request-limit-per-endpoint = 2
    [request-retries]
    max-retries = 6
    backoff-delay = "PT3S"
    failures-warning-threshold = 4
        """.trimIndent(),
      ),
    )
      .isEqualTo(
        TracesToml.ClientApiConfigToml(
          endpoints = listOf(
            "http://traces-api-1:8080/".toURL(),
            "http://traces-api-2:8080/".toURL(),
          ),
          requestLimitPerEndpoint = 2u,
          requestRetries = RequestRetriesToml(
            maxRetries = 6u,
            backoffDelay = 3.seconds,
            failuresWarningThreshold = 4u,
          ),
        ),
      )
  }

  @Test
  fun `should parse traces full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).traces)
      .isEqualTo(config)
  }

  @Test
  fun `should parse traces minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).traces)
      .isEqualTo(configMinimal)
  }
}
