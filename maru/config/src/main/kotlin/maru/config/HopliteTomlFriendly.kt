/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import linea.domain.RetryConfig
import linea.domain.toBlockParameter
import linea.kotlin.assertIs20Bytes
import java.net.URL
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

data class PayloadValidatorDto(
  @param:ConfigSection("Engine API endpoint of the validator's execution-layer node.")
  val engineApiEndpoint: ApiEndpointDto,
  @param:ConfigDoc(
    description = "Whether to validate execution payloads received via the Engine API. Must stay true " +
      "when the qbft section is set: a validator fails to start with payload validation disabled. " +
      "Only followers may set it to false.",
    default = "true",
  )
  val payloadValidationEnabled: Boolean = true,
) {
  fun domainFriendly(): ValidatorElNode =
    ValidatorElNode(
      engineApiEndpoint = engineApiEndpoint.domainFriendly(endlessRetries = true),
      payloadValidationEnabled = payloadValidationEnabled,
    )
}

data class ApiEndpointDto(
  @param:ConfigDoc(
    description = "Engine API endpoint URL of the execution-layer node.",
    example = "http://el-node:8551",
  )
  val endpoint: URL,
  @param:ConfigDoc(
    description = "Optional path to the JWT secret file used for authenticated Engine API calls. " +
      "Omit to disable JWT authentication.",
    example = "/jwt.hex",
  )
  val jwtSecretPath: String? = null,
  @param:ConfigDoc(
    description = "Overall timeout for a single request to this endpoint.",
    default = "PT1M",
  )
  val timeout: Duration = 1.minutes,
) {
  fun domainFriendly(endlessRetries: Boolean = false): ApiEndpointConfig {
    val retries =
      if (endlessRetries) {
        RetryConfig.endlessRetry(
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 3u,
        )
      } else {
        RetryConfig.noRetries
      }
    return ApiEndpointConfig(
      endpoint = endpoint,
      jwtSecretPath = jwtSecretPath,
      requestRetries = retries,
      timeout = timeout,
    )
  }
}

data class QbftOptionsDtoToml(
  @param:ConfigDoc(
    description = "Minimum time spent building a block before proposing it.",
    default = "PT0.5S",
  )
  val minBlockBuildTime: Duration = 500.milliseconds,
  @param:ConfigDoc(
    description = "Maximum number of QBFT messages queued per round.",
    default = "1000",
  )
  val messageQueueLimit: Int = 1000,
  @param:ConfigDoc(
    description = "Optional fixed expiry duration for a QBFT round. Omit to derive it from " +
      "round-expiry-coefficient.",
  )
  val roundExpiry: Duration? = null,
  @param:ConfigDoc(
    description = "Multiplier used to derive each subsequent round's expiry from the previous one.",
    default = "2.0",
  )
  val roundExpiryCoefficient: Double = 2.0,
  @param:ConfigDoc(
    description = "Maximum number of duplicate QBFT messages kept per round.",
    default = "100",
  )
  val duplicateMessageLimit: Int = 100,
  @param:ConfigDoc(
    description = "Maximum number of blocks a future-dated QBFT message may be ahead of the current height.",
    default = "10",
  )
  val futureMessageMaxDistance: Long = 10L,
  @param:ConfigDoc(
    description = "Maximum number of future-dated QBFT messages queued.",
    default = "1000",
  )
  val futureMessagesLimit: Long = 1000L,
  @param:ConfigDoc(
    description = "Fee recipient address for blocks proposed by this validator (20-byte hex).",
    example = "0x0000000000000000000000000000000000000000",
  )
  val feeRecipient: ByteArray,
) {
  fun toDomain(): QbftConfig =
    QbftConfig(
      minBlockBuildTime = minBlockBuildTime,
      messageQueueLimit = messageQueueLimit,
      roundExpiry = roundExpiry,
      roundExpiryCoefficient = roundExpiryCoefficient,
      duplicateMessageLimit = duplicateMessageLimit,
      futureMessageMaxDistance = futureMessageMaxDistance,
      futureMessagesLimit = futureMessagesLimit,
      feeRecipient = feeRecipient,
    )

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as QbftOptionsDtoToml

    if (messageQueueLimit != other.messageQueueLimit) return false
    if (duplicateMessageLimit != other.duplicateMessageLimit) return false
    if (futureMessageMaxDistance != other.futureMessageMaxDistance) return false
    if (futureMessagesLimit != other.futureMessagesLimit) return false
    if (minBlockBuildTime != other.minBlockBuildTime) return false
    if (roundExpiry != other.roundExpiry) return false
    if (roundExpiryCoefficient != other.roundExpiryCoefficient) return false
    if (!feeRecipient.contentEquals(other.feeRecipient)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = messageQueueLimit
    result = 31 * result + duplicateMessageLimit
    result = 31 * result + futureMessageMaxDistance.hashCode()
    result = 31 * result + futureMessagesLimit.hashCode()
    result = 31 * result + minBlockBuildTime.hashCode()
    result = 31 * result + (roundExpiry?.hashCode() ?: 0)
    result = 31 * result + roundExpiryCoefficient.hashCode()
    result = 31 * result + feeRecipient.contentHashCode()
    return result
  }
}

data class DefaultsDtoToml(
  @param:ConfigSection(
    "Fallback L2 endpoint reused by linea.l2-eth-api-endpoint and fork-transition.l2-eth-api-endpoint.",
  )
  val l2EthEndpoint: ApiEndpointDto,
)

data class LineaConfigDtoToml(
  @param:ConfigDoc(
    description = "Address of the Linea rollup contract on L1 (20-byte hex).",
    example = "0x0000000000000000000000000000000000000000",
  )
  val contractAddress: ByteArray,
  @param:ConfigSection(
    "Legacy L1 endpoint; alias for l1-eth-api-endpoint kept for backwards compatibility. " +
      "Deprecated, use l1-eth-api-endpoint.",
    deprecated = true,
    replacement = "linea.l1-eth-api-endpoint",
  )
  val l1EthApi: ApiEndpointDto? = null, // TODO: This is a fallback for backwards compatibility.
  // Remove in the next major release
  @param:ConfigSection("L1 execution-layer API endpoint used to monitor the rollup contract.")
  val l1EthApiEndpoint: ApiEndpointDto? = l1EthApi,
  @param:ConfigDoc(
    description = "Interval between L1 polls for rollup contract events.",
    default = "PT6S",
  )
  val l1PollingInterval: Duration = 6.seconds,
  @param:ConfigDoc(
    description = "L1 block tag treated as the highest finalized block (e.g. finalized, safe, latest).",
    default = "finalized",
  )
  val l1HighestBlockTag: String = "finalized",
  @param:ConfigSection(
    "L2 execution-layer API endpoint used to set the chain head via the Engine API. " +
      "Falls back to defaults.l2-eth-endpoint when omitted.",
  )
  val l2EthApiEndpoint: ApiEndpointDto? = null,
) {
  init {
    contractAddress.assertIs20Bytes("contractAddress")
    require(l1EthApiEndpoint != null) {
      "l1-eth-api-endpoint has to be defined!"
    }
  }

  fun domainFriendly(defaultL2EthApi: ApiEndpointDto?): LineaConfig {
    require(l2EthApiEndpoint != null || defaultL2EthApi != null) {
      "Either default.l2-eth-endpoint or linea.l2-eth-api have to be defined when [linea] section is defined!"
    }
    return LineaConfig(
      contractAddress = contractAddress,
      l1EthApiEndpoint = l1EthApiEndpoint!!.domainFriendly(),
      l1PollingInterval = l1PollingInterval,
      l1HighestBlockTag = l1HighestBlockTag.toBlockParameter(),
      l2EthApiEndpoint = (l2EthApiEndpoint ?: defaultL2EthApi!!).domainFriendly(),
    )
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as LineaConfigDtoToml

    if (!contractAddress.contentEquals(other.contractAddress)) return false
    if (l1EthApi != other.l1EthApi) return false
    if (l1EthApiEndpoint != other.l1EthApiEndpoint) return false
    if (l1PollingInterval != other.l1PollingInterval) return false
    if (l1HighestBlockTag != other.l1HighestBlockTag) return false
    if (l2EthApiEndpoint != other.l2EthApiEndpoint) return false

    return true
  }

  override fun hashCode(): Int {
    var result = contractAddress.contentHashCode()
    result = 31 * result + (l1EthApi?.hashCode() ?: 0)
    result = 31 * result + (l1EthApiEndpoint?.hashCode() ?: 0)
    result = 31 * result + l1PollingInterval.hashCode()
    result = 31 * result + l1HighestBlockTag.hashCode()
    result = 31 * result + (l2EthApiEndpoint?.hashCode() ?: 0)
    return result
  }
}

data class ForkTransitionDtoToml(
  @param:ConfigSection(
    "Optional L2 endpoint used to observe the protocol fork transition. Falls back to " +
      "defaults.l2-eth-endpoint when omitted.",
  )
  val l2EthApiEndpoint: ApiEndpointDto? = null,
  @param:ConfigDoc(
    description = "Interval between polls for the protocol fork transition.",
    default = "PT1S",
  )
  val protocolTransitionPollingInterval: Duration = 1.seconds,
) {
  fun domainFriendly(defaultL2EthApi: ApiEndpointDto?): ForkTransition {
    val defaultedL2EthApiEndpoint = (l2EthApiEndpoint ?: defaultL2EthApi)?.domainFriendly()
    return ForkTransition(
      l2EthApiEndpoint = defaultedL2EthApiEndpoint,
      protocolTransitionPollingInterval = protocolTransitionPollingInterval,
    )
  }
}

data class MaruConfigDtoToml(
  @param:ConfigDoc(
    description = "Whether empty blocks are allowed when proposing and when validating blocks " +
      "(needed in multi-validator networks).",
    default = "false",
  )
  private val allowEmptyBlocks: Boolean = false,
  @param:ConfigSection("Shared defaults reused by linea and fork-transition; currently provides the L2 endpoint.")
  private val defaults: DefaultsDtoToml? = null,
  @param:ConfigSection(
    "Linea-specific settings (L1/L2 endpoints, contract address). Omit on non-Linea networks. " +
      "l1-eth-api is a deprecated alias of l1-eth-api-endpoint; set one of the two. " +
      "l2-eth-api-endpoint falls back to defaults.l2-eth-endpoint when omitted.",
  )
  private val linea: LineaConfigDtoToml? = null,
  @param:ConfigSection("Persistent on-disk state settings.")
  private val persistence: Persistence,
  @param:ConfigSection("QBFT consensus settings. Omit on follower (non-validator) nodes.")
  private val qbft: QbftOptionsDtoToml?,
  @param:ConfigSection("P2P networking settings. Omit to disable P2P.")
  private val p2p: P2PConfig?,
  @param:ConfigSection("Validator execution-layer node settings. Required when qbft is set.")
  private val payloadValidator: PayloadValidatorDto?,
  @param:ConfigDoc(
    description = "Named map of follower execution-layer endpoints. Each entry maps a follower name " +
      "to its engine API endpoint settings.",
  )
  private val followerEngineApis: Map<String, ApiEndpointDto>?,
  @param:ConfigSection("Observability (metrics, health) settings.")
  private val observability: ObservabilityConfig,
  @param:ConfigSection("Maru JSON-RPC API settings.")
  private val api: ApiConfig,
  @param:ConfigSection("Sync settings used while catching up to the chain head.")
  private val syncing: SyncingConfig,
  @param:ConfigSection("Protocol fork transition monitoring settings. Has defaults so the section may be omitted.")
  private val forkTransition: ForkTransitionDtoToml = ForkTransitionDtoToml(),
) {
  fun domainFriendly(): MaruConfig =
    MaruConfig(
      allowEmptyBlocks = allowEmptyBlocks,
      linea = linea?.domainFriendly(defaults?.l2EthEndpoint),
      persistence = persistence,
      qbft = qbft?.toDomain(),
      p2p = p2p,
      validatorElNode = payloadValidator?.domainFriendly(),
      followers = FollowersConfig(
        followers = followerEngineApis?.mapValues { it.value.domainFriendly() } ?: emptyMap(),
      ),
      observability = observability,
      api = api,
      syncing = syncing,
      forkTransition = forkTransition.domainFriendly(defaultL2EthApi = defaults?.l2EthEndpoint),
    )
}
