/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config

import linea.domain.BlockParameter
import linea.domain.RetryConfig
import linea.kotlin.assertIs20Bytes
import maru.extensions.encodeHex
import java.net.InetAddress
import java.net.URL
import java.nio.file.Path
import kotlin.math.max
import kotlin.time.Duration
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds

data class Persistence(
  val dataPath: Path,
  val privateKeyPath: Path = dataPath.resolve("private-key"),
)

data class ApiEndpointConfig(
  val endpoint: URL,
  val jwtSecretPath: String? = null,
  val requestRetries: RetryConfig = RetryConfig.noRetries,
  val timeout: Duration = 1.minutes,
)

data class FollowersConfig(
  val followers: Map<String, ApiEndpointConfig>,
)

data class P2PConfig(
  val ipAddress: String = "127.0.0.1", // default to localhost for security
  val port: UInt = 9000u,
  val staticPeers: List<String> = emptyList(),
  val reconnectDelay: Duration = 5.seconds,
  val maxPeers: Int = 25,
  val maxUnsyncedPeers: Int = max(1, maxPeers / 10),
  val discovery: Discovery? = null,
  val statusUpdate: StatusUpdate = StatusUpdate(),
  val reputation: Reputation = Reputation(),
  val peeringForkMismatchLeewayTime: Duration = 20.seconds,
  val gossiping: Gossiping = Gossiping(),
) {
  init {
    validateIpAddress(ipAddress)
    require(reputation.smallChange > 0) {
      "smallChange must be a positive number"
    }
    require(reputation.largeChange > reputation.smallChange) {
      "largeChange must be greater than smallChange"
    }
  }

  data class Discovery(
    val port: UInt = 9000u,
    val bootnodes: List<String> = emptyList(),
    val refreshInterval: Duration,
    val searchInterval: Duration = 1.seconds,
    val searchTimeout: Duration = 30.seconds,
    val advertisedIp: String? = null,
  ) {
    init {
      advertisedIp?.let { validateIpAddress(it) }
    }
  }

  companion object {
    private fun validateIpAddress(ip: String) {
      require(ip.isNotBlank()) {
        "IP address must not be blank"
      }
      // InetAddress.getByName accepts both IP addresses and hostnames.
      // We need to ensure it's actually an IP address by checking that
      // the parsed address matches the input (no DNS resolution occurred)
      val address = InetAddress.getByName(ip)
      require(address.hostAddress == ip) {
        "Invalid IP address format: $ip"
      }
    }
  }

  data class StatusUpdate(
    val refreshInterval: Duration = 30.seconds,
    val refreshIntervalLeeway: Duration = 5.seconds,
    val timeout: Duration = 10.seconds,
  )

  data class Reputation(
    val capacity: Int = 1024,
    val largeChange: Int = 10,
    val smallChange: Int = 3,
    val disconnectScoreThreshold: Int = -largeChange,
    val maxReputation: Int = 2 * largeChange,
    val cooldownPeriod: Duration = 2.minutes,
    val banPeriod: Duration = 1.hours,
  )

  /**
   * Gossip options wrapping Teku's tech.pegasys.teku.networking.p2p.gossip.config.GossipConfig
   * https://github.com/ethereum/consensus-specs/blob/v0.11.1/specs/phase0/p2p-interface.md#the-gossip-domain-gossipsub
   */
  data class Gossiping(
    val d: Int = 8,
    val dLow: Int = 6,
    val dHigh: Int = d * 2,
    val dLazy: Int = 6,
    val fanoutTTL: Duration = 60.seconds,
    val gossipSize: Int = 3,
    val history: Int = 6,
    val heartbeatInterval: Duration = 700.milliseconds,
    val seenTTL: Duration = 700.milliseconds * 1115,
    val floodPublishMaxMessageSizeThreshold: Int = 1 shl 14, // 16KiB
    val gossipFactor: Double = 0.25,
    val considerPeersAsDirect: Boolean = false,
  )
}

data class ValidatorElNode(
  val engineApiEndpoint: ApiEndpointConfig,
  val payloadValidationEnabled: Boolean,
)

data class QbftConfig(
  val minBlockBuildTime: Duration = 500.milliseconds,
  val messageQueueLimit: Int = 1000,
  val roundExpiry: Duration? = null,
  val roundExpiryCoefficient: Double = 2.0,
  val duplicateMessageLimit: Int = 100,
  val futureMessageMaxDistance: Long = 10L,
  val futureMessagesLimit: Long = 1000L,
  val feeRecipient: ByteArray,
) {
  init {
    feeRecipient.assertIs20Bytes("feeRecipient")
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as QbftConfig

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

  override fun toString(): String =
    "QbftConfig(" +
      "minBlockBuildTime=$minBlockBuildTime, " +
      "messageQueueLimit=$messageQueueLimit, " +
      "roundExpiry=$roundExpiry, " +
      "roundExpiryCoefficient=$roundExpiryCoefficient, " +
      "duplicateMessageLimit=$duplicateMessageLimit, " +
      "futureMessageMaxDistance=$futureMessageMaxDistance, " +
      "futureMessagesLimit=$futureMessagesLimit, " +
      "feeRecipient=${feeRecipient.encodeHex()}" +
      ")"
}

data class ObservabilityConfig(
  val port: UInt = 9545u,
  val prometheusMetricsEnabled: Boolean = true,
  val jvmMetricsEnabled: Boolean = true,
)

data class LineaConfig(
  val contractAddress: ByteArray,
  val l1EthApiEndpoint: ApiEndpointConfig,
  val l1PollingInterval: Duration = 6.seconds,
  val l1HighestBlockTag: BlockParameter = BlockParameter.Tag.FINALIZED,
  val l2EthApiEndpoint: ApiEndpointConfig,
) {
  init {
    contractAddress.assertIs20Bytes("contractAddress")
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as LineaConfig

    if (!contractAddress.contentEquals(other.contractAddress)) return false
    if (l1EthApiEndpoint != other.l1EthApiEndpoint) return false
    if (l1PollingInterval != other.l1PollingInterval) return false
    if (l1HighestBlockTag != other.l1HighestBlockTag) return false
    if (l2EthApiEndpoint != other.l2EthApiEndpoint) return false

    return true
  }

  override fun hashCode(): Int {
    var result = contractAddress.contentHashCode()
    result = 31 * result + l1EthApiEndpoint.hashCode()
    result = 31 * result + l1PollingInterval.hashCode()
    result = 31 * result + l1HighestBlockTag.hashCode()
    result = 31 * result + l2EthApiEndpoint.hashCode()
    return result
  }
}

data class ApiConfig(
  val port: UInt = 5060u,
)

data class SyncingConfig(
  val peerChainHeightPollingInterval: Duration,
  val syncTargetSelection: SyncTargetSelection,
  val elSyncStatusRefreshInterval: Duration? = null,
  val desyncTolerance: ULong = 5UL,
  val download: Download = Download(),
) {
  sealed interface SyncTargetSelection {
    data object Highest : SyncTargetSelection

    data class MostFrequent(
      val peerChainHeightGranularity: UInt,
    ) : SyncTargetSelection {
      init {
        require(peerChainHeightGranularity > 0U) {
          "peerChainHeightGranularity must be higher than 0"
        }
      }
    }
  }

  data class Download(
    val blockRangeRequestTimeout: Duration = 5.seconds,
    val blocksBatchSize: UInt = 100u,
    val blocksParallelism: UInt = 1u,
    val maxRetries: UInt = 5u,
    val backoffDelay: Duration = 1.seconds,
    val useUnconditionalRandomDownloadPeer: Boolean = false,
  )
}

data class ForkTransition(
  val l2EthApiEndpoint: ApiEndpointConfig? = null,
  val protocolTransitionPollingInterval: Duration = 1.seconds,
)

data class MaruConfig(
  val allowEmptyBlocks: Boolean = false,
  val persistence: Persistence,
  val qbft: QbftConfig?,
  val p2p: P2PConfig?,
  val validatorElNode: ValidatorElNode?,
  val followers: FollowersConfig,
  val observability: ObservabilityConfig,
  val linea: LineaConfig? = null,
  val api: ApiConfig,
  val syncing: SyncingConfig,
  val forkTransition: ForkTransition,
  val useVertxTimers: Boolean = false,
) {
  init {
    if (qbft != null) {
      require(validatorElNode != null) {
        "Validator EL node is required when a node is a QBFT Validator"
      }
      require(validatorElNode.payloadValidationEnabled) {
        "When node is a Validator, payload validation must be enabled"
      }
    }
    if (validatorElNode != null) {
      require(
        !followers.followers.values
          .map { it.endpoint }
          .contains(validatorElNode.engineApiEndpoint.endpoint),
      ) {
        "Validator EL node cannot be defined as a follower"
      }
    }
  }
}
