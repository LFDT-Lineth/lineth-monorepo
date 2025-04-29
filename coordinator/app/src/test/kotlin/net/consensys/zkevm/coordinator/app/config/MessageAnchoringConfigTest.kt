package net.consensys.zkevm.coordinator.app.config

import com.sksamuel.hoplite.ConfigLoaderBuilder
import com.sksamuel.hoplite.toml.TomlPropertySource
import linea.domain.BlockParameter
import linea.domain.RetryConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.net.URI
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class MessageAnchoringConfigTest {
  private val l1DefaultEndpoint = URI("http://l1-default-rpc-endpoint:8545").toURL()
  private val l2DefaultEndpoint = URI("http://l2-default-rpc-endpoint:8545").toURL()
  data class Config(
    val messageAnchoring: MessageAnchoringConfigTomlDto = MessageAnchoringConfigTomlDto()
  )

  private fun parseConfig(toml: String): MessageAnchoringConfig {
    return ConfigLoaderBuilder
      .default()
      .addDecoder(BlockParameterDecoder())
      .addSource(TomlPropertySource(toml))
      .build()
      .loadConfigOrThrow<Config>()
      .let {
        it.messageAnchoring.reified(
          l1DefaultEndpoint = l1DefaultEndpoint,
          l2DefaultEndpoint = l2DefaultEndpoint
        )
      }
  }

  @Test
  fun `should parse message anchoroing full config`() {
    val toml = """
      [message-anchoring]
      disabled = true
      l1-endpoint = "http://l1-rpc-endpoint:8545"
      l2-endpoint = "http://l2-rpc-endpoint:8545"
      l1-highest-block-tag="FINALIZED"
      l2-highest-block-tag="LATEST"
      l1-event-polling-interval="PT30S"
      l1-event-polling-timeout="PT6S"
      l1-success-backoff-delay="PT0.1s"
      l1-event-search-block-chunk=123
      message-anchoring-chunck-size=123
      anchoring-tick-interval="PT3S"
      message-queue-capacity=321
      maxMessagesToAnchorPerL2Transaction=54

      [message-anchoring.l1-request-retries]
      max-retries = 10
      timeout = "PT100S"
      backoff-delay = "PT11S"
      failures-warning-threshold = 1
      [message-anchoring.l2-request-retries]
      max-retries = 20
      timeout = "PT200S"
      backoff-delay = "PT21S"
      failures-warning-threshold = 2
    """.trimIndent()

    assertThat(parseConfig(toml))
      .isEqualTo(
        MessageAnchoringConfig(
          disabled = true,
          l1Endpoint = URI("http://l1-rpc-endpoint:8545").toURL(),
          l2Endpoint = URI("http://l2-rpc-endpoint:8545").toURL(),
          l1HighestBlockTag = BlockParameter.Tag.FINALIZED,
          l2HighestBlockTag = BlockParameter.Tag.LATEST,
          l1RequestRetryConfig = RetryConfig(
            maxRetries = 10u,
            timeout = 100.seconds,
            backoffDelay = 11.seconds,
            failuresWarningThreshold = 1u
          ),
          l2RequestRetryConfig = RetryConfig(
            maxRetries = 20u,
            timeout = 200.seconds,
            backoffDelay = 21.seconds,
            failuresWarningThreshold = 2u
          ),
          l1EventPollingInterval = 30.seconds,
          l1EventPollingTimeout = 6.seconds,
          l1SuccessBackoffDelay = 100.milliseconds,
          l1EventSearchBlockChunk = 123u,
          anchoringTickInterval = 3.seconds,
          messageQueueCapacity = 321u,
          maxMessagesToAnchorPerL2Transaction = 54u
        )
      )
  }

  @Test
  fun `should parse message anchoroing with defaults`() {
    val toml = """
      # Nothing configured to return defaults
    """.trimIndent()

    assertThat(parseConfig(toml))
      .isEqualTo(
        MessageAnchoringConfig(
          disabled = false,
          l1Endpoint = URI("http://l1-default-rpc-endpoint:8545").toURL(),
          l2Endpoint = URI("http://l2-default-rpc-endpoint:8545").toURL(),
          l1HighestBlockTag = BlockParameter.Tag.FINALIZED,
          l2HighestBlockTag = BlockParameter.Tag.LATEST,
          l1RequestRetryConfig = RetryConfig(
            maxRetries = null,
            timeout = null,
            backoffDelay = 1.seconds,
            failuresWarningThreshold = 3u
          ),
          l2RequestRetryConfig = RetryConfig(
            maxRetries = null,
            timeout = null,
            backoffDelay = 1.seconds,
            failuresWarningThreshold = 3u
          ),
          l1EventPollingInterval = 12.seconds,
          l1EventPollingTimeout = 6.seconds,
          l1SuccessBackoffDelay = 1.milliseconds,
          l1EventSearchBlockChunk = 1000u,
          anchoringTickInterval = 2.seconds,
          messageQueueCapacity = 10_000u,
          maxMessagesToAnchorPerL2Transaction = 100u
        )
      )
  }
}
