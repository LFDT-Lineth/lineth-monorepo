package net.consensys.zkevm.coordinator.app.config

import linea.domain.BlockParameter
import java.net.URL
import java.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration
import kotlin.time.toKotlinDuration

data class MessageAnchoringConfigTomlDto(
  var disabled: Boolean = false,
  val l1Endpoint: URL? = null, // shall default to L1 endpoint
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  val l1RequestRetries: RequestRetryConfigTomlFriendly = RequestRetryConfigTomlFriendly.endlessRetry(
    backoffDelay = 1.seconds.toJavaDuration(),
    failuresWarningThreshold = 3
  ),
  val l1EventPollingInterval: Duration = 12.seconds.toJavaDuration(),
  val l1EventPollingTimeout: Duration = 6.seconds.toJavaDuration(),
  val l1SuccessBackoffDelay: Duration = 1.milliseconds.toJavaDuration(), // is configurable mostly for testing purposes
  val l1EventSearchBlockChunk: Int = 1000,
  val l2Endpoint: URL? = null,
  val l2HighestBlockTag: BlockParameter = BlockParameter.Tag.LATEST,
  val l2RequestRetries: RequestRetryConfigTomlFriendly = RequestRetryConfigTomlFriendly.endlessRetry(
    backoffDelay = 1.seconds.toJavaDuration(),
    failuresWarningThreshold = 3
  ),
  val anchoringTickInterval: Duration = 2.seconds.toJavaDuration(),
  val messageQueueCapacity: Int = 10_000,
  val maxMessagesToAnchorPerL2Transaction: Int = 100
) {
  init {
    require(messageQueueCapacity > 0) {
      "messageQueueCapacity must be greater than 0"
    }
    require(maxMessagesToAnchorPerL2Transaction >= 1) {
      "maxMessagesToAnchorPerL2Transaction=$maxMessagesToAnchorPerL2Transaction be equal or greater than 1"
    }
    require(l1EventPollingInterval.toMillis() >= 1) {
      "l1EventPollingInterval=$l1EventPollingInterval must be equal or greater than 1ms"
    }
    require(l1EventPollingTimeout.toMillis() >= 1) {
      "l1EventPollingTimeout=$l1EventPollingTimeout must be equal or greater than 1ms"
    }
    require(l1SuccessBackoffDelay.toMillis() >= 1) {
      "l1SuccessBackoffDelay=$l1SuccessBackoffDelay must be equal or greater than 1ms"
    }
    require(l1EventSearchBlockChunk >= 1) {
      "l1EventSearchBlockChunk=$l1EventSearchBlockChunk must be equal or greater than 1"
    }
    require(anchoringTickInterval.toMillis() >= 1) {
      "anchoringTickInterval must be equal or greater than 1ms"
    }
  }

  fun reified(
    l1DefaultEndpoint: URL,
    l2DefaultEndpoint: URL
  ): MessageAnchoringConfig {
    return MessageAnchoringConfig(
      disabled = disabled,
      l1Endpoint = l1Endpoint ?: l1DefaultEndpoint,
      l2Endpoint = l2Endpoint ?: l2DefaultEndpoint,
      l1HighestBlockTag = l1HighestBlockTag,
      l2HighestBlockTag = l2HighestBlockTag,
      l1RequestRetryConfig = l1RequestRetries.asDomain,
      l2RequestRetryConfig = l2RequestRetries.asDomain,
      l1EventPollingInterval = l1EventPollingInterval.toKotlinDuration(),
      l1EventPollingTimeout = l1EventPollingTimeout.toKotlinDuration(),
      l1SuccessBackoffDelay = l1SuccessBackoffDelay.toKotlinDuration(),
      l1EventSearchBlockChunk = l1EventSearchBlockChunk.toUInt(),
      anchoringTickInterval = anchoringTickInterval.toKotlinDuration(),
      messageQueueCapacity = messageQueueCapacity.toUInt(),
      maxMessagesToAnchorPerL2Transaction = maxMessagesToAnchorPerL2Transaction.toUInt()
    )
  }
}

data class MessageAnchoringConfig(
  override var disabled: Boolean,
  val l1Endpoint: URL,
  val l2Endpoint: URL,
  val l1HighestBlockTag: BlockParameter,
  val l2HighestBlockTag: BlockParameter,
  val l1RequestRetryConfig: linea.domain.RetryConfig,
  val l2RequestRetryConfig: linea.domain.RetryConfig,
  val l1EventPollingInterval: kotlin.time.Duration,
  val l1EventPollingTimeout: kotlin.time.Duration,
  val l1SuccessBackoffDelay: kotlin.time.Duration,
  val l1EventSearchBlockChunk: UInt,
  val anchoringTickInterval: kotlin.time.Duration,
  val messageQueueCapacity: UInt,
  val maxMessagesToAnchorPerL2Transaction: UInt
) : FeatureToggleable
